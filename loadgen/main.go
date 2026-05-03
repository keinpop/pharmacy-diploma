// loadgen — лёгкая нагрузка на запущенную систему pharmacy.
//
// По умолчанию работает 15 минут и одновременно дёргает все 4 сервиса
// (auth/inventory/sales/analytics), чтобы метрики на дашбордах Grafana
// показывали ненулевые значения.
//
// Открыть Grafana:   http://localhost:3000  (admin / admin)
// Открыть Prometheus: http://localhost:9090
// Целевой дашборд:   "Pharmacy — Service Overview"
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	mrand "math/rand/v2"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticspb "pharmacy/analytics/gen/analytics"
	authpb "pharmacy/sales/gen/auth"
	inventorypb "pharmacy/sales/gen/inventory"
	salespb "pharmacy/sales/gen/sales"
)

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

func main() {
	var (
		authAddr      = flag.String("auth", "localhost:50051", "auth gRPC address")
		inventoryAddr = flag.String("inventory", "localhost:50053", "inventory gRPC address")
		salesAddr     = flag.String("sales", "localhost:50054", "sales gRPC address")
		analyticsAddr = flag.String("analytics", "localhost:50055", "analytics gRPC address")
		duration      = flag.Duration("duration", 15*time.Minute, "сколько генерировать нагрузку")
		rps           = flag.Int("rps", 7, "целевой RPS на каждого воркера (≈ ops/sec)")
		salesWorkers  = flag.Int("sales-workers", 5, "сколько параллельных воркеров создаёт продажи")
		readWorkers   = flag.Int("read-workers", 2, "сколько воркеров делает read-запросы")
		reportEvery   = flag.Duration("report-every", 30*time.Second, "период вывода прогресса")
	)
	flag.Parse()

	printIntro(*duration)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Connections
	conns, err := dialAll(*authAddr, *inventoryAddr, *salesAddr, *analyticsAddr)
	if err != nil {
		fmt.Printf("%sНе удалось подключиться: %s%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	defer conns.Close()

	// Регистрируем пользователей и каталог продуктов.
	fmt.Printf("\n%s» Подготовка пользователей и каталога…%s\n", colorCyan, colorReset)
	users, products, err := bootstrap(ctx, conns)
	if err != nil {
		fmt.Printf("%sBootstrap failed: %s%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ ready: %d products, %d users%s\n", colorGreen, len(products), 3, colorReset)

	// Контекст с дедлайном на длительность теста.
	loadCtx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()

	stats := &counters{}

	var wg sync.WaitGroup

	// Воркеры на продажи (admin/pharmacist) — основной поток событий.
	for i := 0; i < *salesWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			salesWorker(loadCtx, conns, users, products, *rps, stats, id)
		}(i)
	}

	// Read-воркеры — список продаж/товаров/остатков.
	for i := 0; i < *readWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			readWorker(loadCtx, conns, users, products, *rps, stats, id)
		}(i)
	}

	// Auth воркер — login/validate/logout, генерируя нагрузку на auth.
	wg.Add(1)
	go func() {
		defer wg.Done()
		authWorker(loadCtx, conns, *rps, stats)
	}()

	// Analytics — реже, ~1 раз в 30 секунд: создание отчётов.
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyticsWorker(loadCtx, conns, users, stats)
	}()

	// Inventory write-off / receive — раз в минуту, чтобы росли соответствующие метрики.
	wg.Add(1)
	go func() {
		defer wg.Done()
		inventoryWorker(loadCtx, conns, users, products, stats)
	}()

	// Прогресс.
	wg.Add(1)
	go func() {
		defer wg.Done()
		progress(loadCtx, *reportEvery, stats)
	}()

	wg.Wait()

	fmt.Printf("\n%s%s━━━ Финал ━━━%s\n", colorBold, colorCyan, colorReset)
	stats.print()
	printOutro()
}

func printIntro(d time.Duration) {
	fmt.Printf("%s%spharmacy load generator%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("Длительность: %s\n", d)
	fmt.Println()
	fmt.Printf("%sГде смотреть метрики:%s\n", colorBold, colorReset)
	fmt.Println("  Grafana:    http://localhost:3000   (admin / admin)")
	fmt.Println("  Prometheus: http://localhost:9090")
	fmt.Println("  Дашборд:    Pharmacy → Pharmacy — Service Overview")
	fmt.Println()
	fmt.Printf("%sСырые метрики каждого сервиса:%s\n", colorBold, colorReset)
	fmt.Println("  http://localhost:9101/metrics  (auth)")
	fmt.Println("  http://localhost:9103/metrics  (inventory)")
	fmt.Println("  http://localhost:9104/metrics  (sales)")
	fmt.Println("  http://localhost:9105/metrics  (analytics)")
	fmt.Println()
	fmt.Printf("%sЧто наблюдать на дашборде:%s\n", colorBold, colorReset)
	fmt.Println("  • Tokens issued/revoked, Login attempts (auth)")
	fmt.Println("  • gRPC request rate (все сервисы)")
	fmt.Println("  • Sales created (sales)")
	fmt.Println("  • Stock deductions / Batches received (inventory)")
	fmt.Println("  • Reports completed (analytics)")
	fmt.Println()
	fmt.Printf("%sCtrl+C — остановить досрочно.%s\n", colorGray, colorReset)
}

func printOutro() {
	fmt.Println()
	fmt.Printf("%sНагрузка завершена.%s Графики продолжат показывать данные ещё несколько минут\n",
		colorGreen, colorReset)
	fmt.Println("(retention Prometheus = 15 дней по умолчанию).")
	fmt.Println()
	fmt.Printf("Открой Grafana: %shttp://localhost:3000%s\n", colorCyan, colorReset)
	fmt.Println("  Login:   admin")
	fmt.Println("  Pass:    admin (можно пропустить смену пароля)")
	fmt.Println("  Дашборд: Dashboards → Pharmacy → Pharmacy — Service Overview")
}

// Connections ─────────────────────────────────────────────────────────────────

type connections struct {
	auth      *grpc.ClientConn
	inventory *grpc.ClientConn
	sales     *grpc.ClientConn
	analytics *grpc.ClientConn

	authClient      authpb.AuthServiceClient
	inventoryClient inventorypb.InventoryServiceClient
	salesClient     salespb.SalesServiceClient
	analyticsClient analyticspb.AnalyticsServiceClient
}

func (c *connections) Close() {
	for _, cn := range []*grpc.ClientConn{c.auth, c.inventory, c.sales, c.analytics} {
		if cn != nil {
			_ = cn.Close()
		}
	}
}

func dialAll(authAddr, invAddr, salesAddr, anAddr string) (*connections, error) {
	dial := func(addr string) (*grpc.ClientConn, error) {
		conn, err := grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, err
		}
		conn.Connect()
		return conn, nil
	}
	c := &connections{}
	var err error
	if c.auth, err = dial(authAddr); err != nil {
		return nil, fmt.Errorf("dial auth: %w", err)
	}
	if c.inventory, err = dial(invAddr); err != nil {
		return nil, fmt.Errorf("dial inventory: %w", err)
	}
	if c.sales, err = dial(salesAddr); err != nil {
		return nil, fmt.Errorf("dial sales: %w", err)
	}
	if c.analytics, err = dial(anAddr); err != nil {
		return nil, fmt.Errorf("dial analytics: %w", err)
	}
	c.authClient = authpb.NewAuthServiceClient(c.auth)
	c.inventoryClient = inventorypb.NewInventoryServiceClient(c.inventory)
	c.salesClient = salespb.NewSalesServiceClient(c.sales)
	c.analyticsClient = analyticspb.NewAnalyticsServiceClient(c.analytics)
	return c, nil
}

// Bootstrap ───────────────────────────────────────────────────────────────────

type loadUsers struct {
	adminToken      string
	managerToken    string
	pharmacistToken string
	pharmacistName  string

	loginUsername string
	loginPassword string
}

func bootstrap(ctx context.Context, c *connections) (loadUsers, []string, error) {
	suffix := randomSuffix()
	register := func(role string) (string, string, error) {
		username := fmt.Sprintf("load_%s_%s", role, suffix)
		password := "load-p@ssw0rd"
		ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		resp, err := c.authClient.Register(ctx2, &authpb.RegisterRequest{
			Username: username,
			Password: password,
			Role:     role,
		})
		if err != nil {
			return "", "", err
		}
		return resp.GetToken(), username, nil
	}

	u := loadUsers{}
	var err error
	if u.adminToken, _, err = register("admin"); err != nil {
		return u, nil, fmt.Errorf("register admin: %w", err)
	}
	if u.managerToken, _, err = register("manager"); err != nil {
		return u, nil, fmt.Errorf("register manager: %w", err)
	}
	var pharmaName string
	if u.pharmacistToken, pharmaName, err = register("pharmacist"); err != nil {
		return u, nil, fmt.Errorf("register pharmacist: %w", err)
	}
	u.pharmacistName = pharmaName

	// Дополнительный пользователь для login-нагрузки.
	loginToken, loginUser, err := register("manager")
	if err == nil {
		_ = loginToken
		u.loginUsername = loginUser
		u.loginPassword = "load-p@ssw0rd"
	}

	// Каталог
	products := []*inventorypb.CreateProductRequest{
		{Name: "Аспирин-LOAD-" + suffix, ActiveSubstance: "аск", Form: "таблетка", Dosage: "100мг", Category: "otc", Unit: "шт", ReorderPoint: 10, TherapeuticGroup: "cardiovascular"},
		{Name: "Парацетамол-LOAD-" + suffix, ActiveSubstance: "парацетамол", Form: "таблетка", Dosage: "500мг", Category: "otc", Unit: "шт", ReorderPoint: 20, TherapeuticGroup: "painkiller"},
		{Name: "Лоратадин-LOAD-" + suffix, ActiveSubstance: "лоратадин", Form: "таблетка", Dosage: "10мг", Category: "otc", Unit: "шт", ReorderPoint: 5, TherapeuticGroup: "antihistamine"},
		{Name: "Ибупрофен-LOAD-" + suffix, ActiveSubstance: "ибу", Form: "таблетка", Dosage: "200мг", Category: "otc", Unit: "шт", ReorderPoint: 10, TherapeuticGroup: "painkiller"},
	}

	ids := make([]string, 0, len(products))
	for _, p := range products {
		ctx2, cancel := context.WithTimeout(withToken(ctx, u.adminToken), 10*time.Second)
		resp, err := c.inventoryClient.CreateProduct(ctx2, p)
		cancel()
		if err != nil {
			fmt.Printf("%s  warn: create product %q: %s%s\n", colorYellow, p.Name, err, colorReset)
			continue
		}
		ids = append(ids, resp.GetProduct().GetId())

		// Сразу же приёмка одной партии, чтобы было что продавать.
		ctx3, cancel3 := context.WithTimeout(withToken(ctx, u.pharmacistToken), 10*time.Second)
		_, err = c.inventoryClient.ReceiveBatch(ctx3, &inventorypb.ReceiveBatchRequest{
			ProductId:     resp.GetProduct().GetId(),
			SeriesNumber:  fmt.Sprintf("LOAD-%s-%d", suffix, len(ids)),
			ExpiresAt:     timestamppb.New(time.Now().AddDate(0, 12, 0)),
			Quantity:      9999,
			PurchasePrice: 50,
			RetailPrice:   100,
		})
		cancel3()
		if err != nil {
			fmt.Printf("%s  warn: receive batch %q: %s%s\n", colorYellow, p.Name, err, colorReset)
		}
	}
	if len(ids) == 0 {
		return u, nil, fmt.Errorf("ни один продукт не создан")
	}
	return u, ids, nil
}

// Workers ─────────────────────────────────────────────────────────────────────

type counters struct {
	sales     atomic.Int64
	salesErrs atomic.Int64

	reads     atomic.Int64
	readsErrs atomic.Int64

	auths     atomic.Int64
	authsErrs atomic.Int64

	reports     atomic.Int64
	reportsErrs atomic.Int64

	invOps     atomic.Int64
	invOpsErrs atomic.Int64
}

func (s *counters) print() {
	fmt.Printf("  %ssales%s     ok=%d err=%d\n", colorBold, colorReset, s.sales.Load(), s.salesErrs.Load())
	fmt.Printf("  %sreads%s     ok=%d err=%d\n", colorBold, colorReset, s.reads.Load(), s.readsErrs.Load())
	fmt.Printf("  %sauth ops%s  ok=%d err=%d\n", colorBold, colorReset, s.auths.Load(), s.authsErrs.Load())
	fmt.Printf("  %sreports%s   ok=%d err=%d\n", colorBold, colorReset, s.reports.Load(), s.reportsErrs.Load())
	fmt.Printf("  %sinventory%s ok=%d err=%d\n", colorBold, colorReset, s.invOps.Load(), s.invOpsErrs.Load())
}

func progress(ctx context.Context, every time.Duration, s *counters) {
	t := time.NewTicker(every)
	defer t.Stop()
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			elapsed := time.Since(start).Round(time.Second)
			fmt.Printf("\n%s[%s]%s ", colorGray, elapsed, colorReset)
			fmt.Printf("sales=%d reads=%d auth=%d reports=%d inv=%d   errs: s=%d r=%d a=%d rep=%d i=%d\n",
				s.sales.Load(), s.reads.Load(), s.auths.Load(), s.reports.Load(), s.invOps.Load(),
				s.salesErrs.Load(), s.readsErrs.Load(), s.authsErrs.Load(), s.reportsErrs.Load(), s.invOpsErrs.Load(),
			)
		}
	}
}

func salesWorker(ctx context.Context, c *connections, u loadUsers, products []string, rps int, s *counters, id int) {
	if rps <= 0 {
		rps = 1
	}
	interval := time.Second / time.Duration(rps)
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		productID := products[mrand.IntN(len(products))]
		quantity := int32(1 + mrand.IntN(3))
		ctx2, cancel := context.WithTimeout(withToken(ctx, u.pharmacistToken), 5*time.Second)
		_, err := c.salesClient.CreateSale(ctx2, &salespb.CreateSaleRequest{
			Items: []*salespb.SaleItemRequest{{ProductId: productID, Quantity: quantity}},
		})
		cancel()
		if err != nil {
			s.salesErrs.Add(1)
		} else {
			s.sales.Add(1)
		}
		_ = id
	}
}

func readWorker(ctx context.Context, c *connections, u loadUsers, products []string, rps int, s *counters, id int) {
	if rps <= 0 {
		rps = 1
	}
	interval := time.Second / time.Duration(rps)
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		// Чередуем разные read-запросы.
		switch mrand.IntN(4) {
		case 0:
			ctx2, cancel := context.WithTimeout(withToken(ctx, u.managerToken), 5*time.Second)
			_, err := c.salesClient.ListSales(ctx2, &salespb.ListSalesRequest{Page: 1, PageSize: 20})
			cancel()
			recordRead(s, err)
		case 1:
			productID := products[mrand.IntN(len(products))]
			ctx2, cancel := context.WithTimeout(withToken(ctx, u.managerToken), 5*time.Second)
			_, err := c.inventoryClient.GetStock(ctx2, &inventorypb.GetStockRequest{ProductId: productID})
			cancel()
			recordRead(s, err)
		case 2:
			ctx2, cancel := context.WithTimeout(withToken(ctx, u.managerToken), 5*time.Second)
			_, err := c.inventoryClient.ListProducts(ctx2, &inventorypb.ListProductsRequest{Page: 1, PageSize: 20})
			cancel()
			recordRead(s, err)
		case 3:
			ctx2, cancel := context.WithTimeout(withToken(ctx, u.managerToken), 5*time.Second)
			_, err := c.inventoryClient.ListLowStock(ctx2, &inventorypb.ListLowStockRequest{})
			cancel()
			recordRead(s, err)
		}
		_ = id
	}
}

func recordRead(s *counters, err error) {
	if err != nil {
		s.readsErrs.Add(1)
	} else {
		s.reads.Add(1)
	}
}

func authWorker(ctx context.Context, c *connections, rps int, s *counters) {
	if rps <= 0 {
		rps = 1
	}
	interval := time.Second / time.Duration(max(1, rps/2))
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		// Регистрируем рандомного пользователя и валидируем токен — это создаёт
		// нагрузку на login_attempts/tokens_issued/tokens_revoked.
		username := "load_eph_" + randomSuffix()
		ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
		reg, err := c.authClient.Register(ctx2, &authpb.RegisterRequest{
			Username: username,
			Password: "ephemeral-pass",
			Role:     "pharmacist",
		})
		cancel()
		if err != nil {
			s.authsErrs.Add(1)
			continue
		}
		s.auths.Add(1)

		// Login → сразу же снова получаем токен.
		ctx3, cancel3 := context.WithTimeout(ctx, 5*time.Second)
		_, lerr := c.authClient.Login(ctx3, &authpb.LoginRequest{Username: username, Password: "ephemeral-pass"})
		cancel3()
		if lerr != nil {
			s.authsErrs.Add(1)
		} else {
			s.auths.Add(1)
		}

		// Validate
		ctx4, cancel4 := context.WithTimeout(ctx, 5*time.Second)
		_, verr := c.authClient.ValidateToken(ctx4, &authpb.ValidateTokenRequest{Token: reg.GetToken()})
		cancel4()
		if verr != nil {
			s.authsErrs.Add(1)
		} else {
			s.auths.Add(1)
		}

		// Logout
		ctx5, cancel5 := context.WithTimeout(ctx, 5*time.Second)
		_, lgerr := c.authClient.Logout(ctx5, &authpb.LogoutRequest{Token: reg.GetToken()})
		cancel5()
		if lgerr != nil {
			s.authsErrs.Add(1)
		} else {
			s.auths.Add(1)
		}
	}
}

func analyticsWorker(ctx context.Context, c *connections, u loadUsers, s *counters) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()

	periods := []analyticspb.ReportPeriod{
		analyticspb.ReportPeriod_MONTH,
		analyticspb.ReportPeriod_HALF_YEAR,
		analyticspb.ReportPeriod_YEAR,
	}
	lookbacks := []int32{1, 6, 12}
	tick := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		ctx2, cancel := context.WithTimeout(withToken(ctx, u.managerToken), 10*time.Second)
		var err error
		switch tick % 4 {
		case 0:
			_, err = c.analyticsClient.CreateSalesReport(ctx2, &analyticspb.CreateSalesReportRequest{
				Period: periods[tick%len(periods)],
			})
		case 1:
			_, err = c.analyticsClient.CreateForecast(ctx2, &analyticspb.CreateForecastRequest{
				LookbackMonths: lookbacks[tick%len(lookbacks)],
			})
		case 2:
			_, err = c.analyticsClient.CreateWriteOffReport(ctx2, &analyticspb.CreateWriteOffReportRequest{
				Period: periods[tick%len(periods)],
			})
		case 3:
			_, err = c.analyticsClient.CreateWasteAnalysis(ctx2, &analyticspb.CreateWasteAnalysisRequest{
				Period: periods[tick%len(periods)],
			})
		}
		cancel()
		if err != nil {
			s.reportsErrs.Add(1)
		} else {
			s.reports.Add(1)
		}
		tick++
	}
}

func inventoryWorker(ctx context.Context, c *connections, u loadUsers, products []string, s *counters) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		// Раз в минуту вызываем write-off expired (даже если ничего не списать —
		// это пинг по DB, метрика gRPC растёт) и регистрируем новую партию.
		ctx2, cancel := context.WithTimeout(withToken(ctx, u.adminToken), 10*time.Second)
		_, err := c.inventoryClient.WriteOffExpired(ctx2, &inventorypb.WriteOffExpiredRequest{})
		cancel()
		if err != nil {
			s.invOpsErrs.Add(1)
		} else {
			s.invOps.Add(1)
		}

		productID := products[mrand.IntN(len(products))]
		ctx3, cancel3 := context.WithTimeout(withToken(ctx, u.pharmacistToken), 10*time.Second)
		_, err = c.inventoryClient.ReceiveBatch(ctx3, &inventorypb.ReceiveBatchRequest{
			ProductId:     productID,
			SeriesNumber:  fmt.Sprintf("LOAD-RB-%s", randomSuffix()),
			ExpiresAt:     timestamppb.New(time.Now().AddDate(0, 6, 0)),
			Quantity:      randInt(50, 200),
			PurchasePrice: 50,
			RetailPrice:   100,
		})
		cancel3()
		if err != nil {
			s.invOpsErrs.Add(1)
		} else {
			s.invOps.Add(1)
		}
	}
}

// Helpers ─────────────────────────────────────────────────────────────────────

func withToken(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}

func randomSuffix() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func randInt(lo, hi int) int32 {
	if hi <= lo {
		return int32(lo)
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(hi-lo)))
	if err != nil {
		return int32(lo)
	}
	return int32(lo) + int32(n.Int64())
}
