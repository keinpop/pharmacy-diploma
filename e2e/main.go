// e2e — сценарный smoke-тест системы pharmacy.
//
// Скрипт подключается к запущенным контейнерам по gRPC, прогоняет основные
// сценарии (auth → inventory → sales → analytics), печатает результаты в виде
// таблицы и продолжает выполнение даже при mismatch'ах.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticspb "pharmacy/analytics/gen/analytics"
	authpb "pharmacy/sales/gen/auth"
	inventorypb "pharmacy/sales/gen/inventory"
	salespb "pharmacy/sales/gen/sales"
)

// Configuration ───────────────────────────────────────────────────────────────

type config struct {
	authAddr      string
	inventoryAddr string
	salesAddr     string
	analyticsAddr string
	salesCount    int
	pollTimeout   time.Duration
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.authAddr, "auth", "localhost:50051", "auth gRPC address")
	flag.StringVar(&cfg.inventoryAddr, "inventory", "localhost:50053", "inventory gRPC address")
	flag.StringVar(&cfg.salesAddr, "sales", "localhost:50054", "sales gRPC address")
	flag.StringVar(&cfg.analyticsAddr, "analytics", "localhost:50055", "analytics gRPC address")
	flag.IntVar(&cfg.salesCount, "sales-count", 6, "сколько продаж сгенерировать для аналитики")
	flag.DurationVar(&cfg.pollTimeout, "poll-timeout", 30*time.Second, "сколько ждать готовности отчётов аналитики")
	flag.Parse()
	return cfg
}

// Reporting ───────────────────────────────────────────────────────────────────

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

type stepResult struct {
	scenario string
	step     string
	ok       bool
	detail   string
	skipped  bool
}

type runner struct {
	results []stepResult
	current string
}

func (r *runner) scenario(name string) {
	r.current = name
	fmt.Printf("\n%s%s━━━ %s ━━━%s\n", colorBold, colorCyan, name, colorReset)
}

func (r *runner) record(step string, err error, detail string) bool {
	res := stepResult{scenario: r.current, step: step, ok: err == nil, detail: detail}
	if err != nil {
		res.detail = fmt.Sprintf("%s: %s", detail, prettyErr(err))
	}
	r.results = append(r.results, res)
	if err != nil {
		fmt.Printf("  %s✗%s %s — %s%s%s\n", colorRed, colorReset, step, colorRed, prettyErr(err), colorReset)
	} else if detail != "" {
		fmt.Printf("  %s✓%s %s %s(%s)%s\n", colorGreen, colorReset, step, colorGray, detail, colorReset)
	} else {
		fmt.Printf("  %s✓%s %s\n", colorGreen, colorReset, step)
	}
	return err == nil
}

func (r *runner) skip(step, reason string) {
	r.results = append(r.results, stepResult{scenario: r.current, step: step, skipped: true, detail: reason})
	fmt.Printf("  %s∘%s %s %s(%s)%s\n", colorYellow, colorReset, step, colorGray, reason, colorReset)
}

func (r *runner) note(msg string) {
	fmt.Printf("  %s⚠ %s%s\n", colorYellow, msg, colorReset)
}

func (r *runner) summary() int {
	var total, passed, failed, skipped int
	for _, res := range r.results {
		total++
		switch {
		case res.skipped:
			skipped++
		case res.ok:
			passed++
		default:
			failed++
		}
	}

	fmt.Printf("\n%s%s━━━ Итог ━━━%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("  Всего шагов:  %s%d%s\n", colorBold, total, colorReset)
	fmt.Printf("  Успех:        %s%d%s\n", colorGreen, passed, colorReset)
	fmt.Printf("  Падений:      %s%d%s\n", colorRed, failed, colorReset)
	fmt.Printf("  Пропущено:    %s%d%s\n", colorYellow, skipped, colorReset)

	if failed > 0 {
		fmt.Printf("\n%sПроваленные шаги:%s\n", colorBold, colorReset)
		for _, res := range r.results {
			if !res.ok && !res.skipped {
				fmt.Printf("  %s•%s [%s] %s — %s\n",
					colorRed, colorReset, res.scenario, res.step, res.detail)
			}
		}
		return 1
	}
	return 0
}

func prettyErr(err error) string {
	if err == nil {
		return ""
	}
	if st, ok := status.FromError(err); ok {
		return fmt.Sprintf("%s: %s", st.Code(), st.Message())
	}
	return err.Error()
}

// Helpers ─────────────────────────────────────────────────────────────────────

func dial(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", addr, err)
	}
	// Triggers a connection attempt right now, иначе NewClient ленив.
	conn.Connect()
	return conn, nil
}

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

// Scenarios ───────────────────────────────────────────────────────────────────

type clients struct {
	auth      authpb.AuthServiceClient
	inventory inventorypb.InventoryServiceClient
	sales     salespb.SalesServiceClient
	analytics analyticspb.AnalyticsServiceClient
}

type users struct {
	adminToken      string
	managerToken    string
	pharmacistToken string
	pharmacistName  string
}

func main() {
	cfg := parseFlags()

	fmt.Printf("%s%spharmacy E2E smoke%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%sauth%s=%s  %sinventory%s=%s  %ssales%s=%s  %sanalytics%s=%s\n",
		colorGray, colorReset, cfg.authAddr,
		colorGray, colorReset, cfg.inventoryAddr,
		colorGray, colorReset, cfg.salesAddr,
		colorGray, colorReset, cfg.analyticsAddr,
	)

	r := &runner{}

	// 0. Connections
	r.scenario("0. Подключение к сервисам")
	authConn, err := dial(cfg.authAddr)
	if !r.record("dial auth", err, cfg.authAddr) {
		os.Exit(r.summary())
	}
	defer func() { _ = authConn.Close() }()

	invConn, err := dial(cfg.inventoryAddr)
	if !r.record("dial inventory", err, cfg.inventoryAddr) {
		os.Exit(r.summary())
	}
	defer func() { _ = invConn.Close() }()

	salesConn, err := dial(cfg.salesAddr)
	if !r.record("dial sales", err, cfg.salesAddr) {
		os.Exit(r.summary())
	}
	defer func() { _ = salesConn.Close() }()

	anConn, err := dial(cfg.analyticsAddr)
	if !r.record("dial analytics", err, cfg.analyticsAddr) {
		os.Exit(r.summary())
	}
	defer func() { _ = anConn.Close() }()

	c := &clients{
		auth:      authpb.NewAuthServiceClient(authConn),
		inventory: inventorypb.NewInventoryServiceClient(invConn),
		sales:     salespb.NewSalesServiceClient(salesConn),
		analytics: analyticspb.NewAnalyticsServiceClient(anConn),
	}

	u := scenarioAuth(r, c)
	if u.adminToken == "" {
		r.note("Нет admin-токена → дальнейшие сценарии будут падать на авторизации.")
	}

	productIDs := scenarioInventoryCatalog(r, c, u)
	scenarioInventoryStock(r, c, u, productIDs)
	scenarioSales(r, c, u, productIDs, cfg.salesCount)
	scenarioInventoryReports(r, c, u)
	scenarioAnalytics(r, c, u, cfg.pollTimeout)
	scenarioLogout(r, c, u)

	os.Exit(r.summary())
}

// 1. Auth scenarios ───────────────────────────────────────────────────────────

func scenarioAuth(r *runner, c *clients) users {
	r.scenario("1. Auth: регистрация / логин / валидация")

	suffix := randomSuffix()
	usersInfo := users{}

	register := func(role, label string) string {
		username := fmt.Sprintf("e2e_%s_%s", role, suffix)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := c.auth.Register(ctx, &authpb.RegisterRequest{
			Username: username,
			Password: "p@ssw0rd-e2e",
			Role:     role,
		})
		if !r.record(fmt.Sprintf("register %s", label), err,
			fmt.Sprintf("user=%s id=%d", username, resp.GetUserId())) {
			return ""
		}
		if role == "pharmacist" {
			usersInfo.pharmacistName = username
		}
		return resp.GetToken()
	}

	usersInfo.adminToken = register("admin", "admin")
	usersInfo.managerToken = register("manager", "manager")
	usersInfo.pharmacistToken = register("pharmacist", "pharmacist")

	if usersInfo.adminToken != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := c.auth.ValidateToken(ctx, &authpb.ValidateTokenRequest{Token: usersInfo.adminToken})
		r.record("validate admin token", err, fmt.Sprintf("username=%s role=%s", resp.GetUsername(), resp.GetRole()))
		if err == nil && resp.GetRole() != "admin" {
			r.note(fmt.Sprintf("ожидалась роль admin, получили %q", resp.GetRole()))
		}
	}

	// Login сценарий — проверяем, что повторный логин даёт валидный токен.
	if usersInfo.pharmacistName != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := c.auth.Login(ctx, &authpb.LoginRequest{
			Username: usersInfo.pharmacistName,
			Password: "p@ssw0rd-e2e",
		})
		if r.record("login pharmacist (повторный)", err, fmt.Sprintf("role=%s", resp.GetRole())) {
			// Используем свежий токен — старый продолжит работать тоже, но это безопаснее.
			usersInfo.pharmacistToken = resp.GetToken()
		}
	}

	// Невалидный токен.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.auth.ValidateToken(ctx, &authpb.ValidateTokenRequest{Token: "definitely-not-a-jwt"})
	if err == nil {
		r.record("validate невалидного токена", errors.New("expected error, got success"), "")
	} else {
		r.record("validate невалидного токена", nil, "ожидаемо отклонён")
	}

	return usersInfo
}

// 2. Inventory: каталог продуктов ─────────────────────────────────────────────

func scenarioInventoryCatalog(r *runner, c *clients, u users) []string {
	r.scenario("2. Inventory: каталог товаров (admin)")

	if u.adminToken == "" {
		r.skip("create products", "нет admin-токена")
		return nil
	}

	suffix := randomSuffix()
	products := []*inventorypb.CreateProductRequest{
		{
			Name:             "Аспирин-E2E-" + suffix,
			TradeName:        "Aspirin Cardio",
			ActiveSubstance:  "ацетилсалициловая кислота",
			Form:             "таблетка",
			Dosage:           "100мг",
			Category:         "otc",
			Unit:             "шт",
			ReorderPoint:     10,
			TherapeuticGroup: "cardiovascular",
		},
		{
			Name:             "Парацетамол-E2E-" + suffix,
			TradeName:        "Panadol",
			ActiveSubstance:  "парацетамол",
			Form:             "таблетка",
			Dosage:           "500мг",
			Category:         "otc",
			Unit:             "шт",
			ReorderPoint:     20,
			TherapeuticGroup: "painkiller",
		},
		{
			Name:             "Лоратадин-E2E-" + suffix,
			TradeName:        "Кларитин",
			ActiveSubstance:  "лоратадин",
			Form:             "таблетка",
			Dosage:           "10мг",
			Category:         "otc",
			Unit:             "шт",
			ReorderPoint:     5,
			TherapeuticGroup: "antihistamine",
		},
	}

	ids := make([]string, 0, len(products))
	for i, req := range products {
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.adminToken), 5*time.Second)
		resp, err := c.inventory.CreateProduct(ctx, req)
		cancel()
		label := fmt.Sprintf("create product #%d", i+1)
		if r.record(label, err, fmt.Sprintf("name=%s id=%s", req.Name, resp.GetProduct().GetId())) {
			ids = append(ids, resp.GetProduct().GetId())
		}
	}

	if len(ids) > 0 {
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.adminToken), 5*time.Second)
		got, err := c.inventory.GetProduct(ctx, &inventorypb.GetProductRequest{Id: ids[0]})
		cancel()
		r.record("get product by id", err, fmt.Sprintf("name=%s", got.GetProduct().GetName()))
	}

	{
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.adminToken), 5*time.Second)
		list, err := c.inventory.ListProducts(ctx, &inventorypb.ListProductsRequest{Page: 1, PageSize: 100})
		cancel()
		r.record("list products", err, fmt.Sprintf("total=%d", list.GetTotal()))
	}

	// Дадим Elastic 1 секунду, чтобы успеть проиндексировать.
	time.Sleep(1 * time.Second)
	{
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.adminToken), 5*time.Second)
		search, err := c.inventory.SearchProducts(ctx, &inventorypb.SearchProductsRequest{Query: "E2E-" + suffix, Limit: 10})
		cancel()
		detail := fmt.Sprintf("hits=%d", len(search.GetProducts()))
		r.record("search products (ES)", err, detail)
		if err == nil && len(search.GetProducts()) == 0 {
			r.note("Elastic не вернул результатов — возможно индексация ещё не завершилась")
		}
	}

	return ids
}

// 3. Inventory: батчи и склад ─────────────────────────────────────────────────

func scenarioInventoryStock(r *runner, c *clients, u users, productIDs []string) {
	r.scenario("3. Inventory: партии и остатки (pharmacist)")

	if u.pharmacistToken == "" {
		r.skip("receive batches", "нет токена pharmacist")
		return
	}
	if len(productIDs) == 0 {
		r.skip("receive batches", "нет созданных продуктов")
		return
	}

	for i, pid := range productIDs {
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.pharmacistToken), 5*time.Second)
		// Большая партия со сроком +6 месяцев.
		req := &inventorypb.ReceiveBatchRequest{
			ProductId:     pid,
			SeriesNumber:  fmt.Sprintf("E2E-%s-%d", randomSuffix(), i),
			ExpiresAt:     timestamppb.New(time.Now().AddDate(0, 6, 0)),
			Quantity:      200,
			PurchasePrice: 50.0 + float64(i)*10,
			RetailPrice:   100.0 + float64(i)*10,
		}
		resp, err := c.inventory.ReceiveBatch(ctx, req)
		cancel()
		r.record(fmt.Sprintf("receive batch product=%s", short(pid)), err,
			fmt.Sprintf("qty=%d batch=%s", req.Quantity, short(resp.GetBatch().GetId())))
	}

	// А ещё — небольшая партия с почти истёкшим сроком, чтобы было что списать.
	if len(productIDs) > 0 {
		pid := productIDs[len(productIDs)-1]
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.pharmacistToken), 5*time.Second)
		_, err := c.inventory.ReceiveBatch(ctx, &inventorypb.ReceiveBatchRequest{
			ProductId:     pid,
			SeriesNumber:  fmt.Sprintf("E2E-EXPSOON-%s", randomSuffix()),
			ExpiresAt:     timestamppb.New(time.Now().AddDate(0, 0, 7)),
			Quantity:      5,
			PurchasePrice: 30.0,
			RetailPrice:   60.0,
		})
		cancel()
		r.record("receive expiring-soon batch", err, "qty=5 expires_in=7d")
	}

	for _, pid := range productIDs {
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.pharmacistToken), 5*time.Second)
		resp, err := c.inventory.GetStock(ctx, &inventorypb.GetStockRequest{ProductId: pid})
		cancel()
		stock := resp.GetStock()
		r.record(fmt.Sprintf("get stock product=%s", short(pid)), err,
			fmt.Sprintf("total=%d available=%d", stock.GetTotalQuantity(), stock.GetAvailable()))
	}
}

// 4. Sales: создание продаж ───────────────────────────────────────────────────

func scenarioSales(r *runner, c *clients, u users, productIDs []string, want int) {
	r.scenario(fmt.Sprintf("4. Sales: создание %d продаж (pharmacist)", want))

	if u.pharmacistToken == "" {
		r.skip("create sale", "нет токена pharmacist")
		return
	}
	if len(productIDs) == 0 {
		r.skip("create sale", "нет продуктов")
		return
	}

	created := 0
	var firstSaleID string
	for i := 0; i < want; i++ {
		// Чередуем продукты и количество, чтобы у аналитики было разнообразие.
		productID := productIDs[i%len(productIDs)]
		quantity := int32(1 + i%3)
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.pharmacistToken), 5*time.Second)
		resp, err := c.sales.CreateSale(ctx, &salespb.CreateSaleRequest{
			Items: []*salespb.SaleItemRequest{
				{ProductId: productID, Quantity: quantity},
			},
		})
		cancel()
		label := fmt.Sprintf("create sale #%d", i+1)
		ok := r.record(label, err, fmt.Sprintf("seller=%s qty=%d total=%.2f",
			resp.GetSale().GetSellerUsername(), quantity, resp.GetSale().GetTotalAmount()))
		if ok {
			if firstSaleID == "" {
				firstSaleID = resp.GetSale().GetId()
			}
			if u.pharmacistName != "" && resp.GetSale().GetSellerUsername() != u.pharmacistName {
				r.note(fmt.Sprintf("ожидался seller_username=%s, получили %s",
					u.pharmacistName, resp.GetSale().GetSellerUsername()))
			}
			created++
		}
	}

	if firstSaleID != "" {
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.pharmacistToken), 5*time.Second)
		got, err := c.sales.GetSale(ctx, &salespb.GetSaleRequest{Id: firstSaleID})
		cancel()
		r.record("get sale by id", err, fmt.Sprintf("items=%d total=%.2f",
			len(got.GetSale().GetItems()), got.GetSale().GetTotalAmount()))
	}

	{
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.pharmacistToken), 5*time.Second)
		list, err := c.sales.ListSales(ctx, &salespb.ListSalesRequest{Page: 1, PageSize: 100})
		cancel()
		r.record("list sales", err, fmt.Sprintf("total=%d created_in_run=%d", list.GetTotal(), created))
	}
}

// 5. Inventory отчёты ─────────────────────────────────────────────────────────

func scenarioInventoryReports(r *runner, c *clients, u users) {
	r.scenario("5. Inventory: отчёты по партиям/остаткам")

	if u.adminToken == "" {
		r.skip("inventory reports", "нет admin-токена")
		return
	}

	{
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.adminToken), 5*time.Second)
		resp, err := c.inventory.ListExpiringBatches(ctx, &inventorypb.ListExpiringRequest{DaysAhead: 30})
		cancel()
		r.record("list expiring batches", err, fmt.Sprintf("count=%d", len(resp.GetBatches())))
	}

	{
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.adminToken), 5*time.Second)
		resp, err := c.inventory.ListLowStock(ctx, &inventorypb.ListLowStockRequest{})
		cancel()
		r.record("list low stock", err, fmt.Sprintf("count=%d", len(resp.GetItems())))
	}

	{
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.adminToken), 5*time.Second)
		resp, err := c.inventory.WriteOffExpired(ctx, &inventorypb.WriteOffExpiredRequest{})
		cancel()
		r.record("write off expired", err, fmt.Sprintf("written_off=%d", resp.GetWrittenOffCount()))
	}
}

// 6. Analytics ────────────────────────────────────────────────────────────────

func scenarioAnalytics(r *runner, c *clients, u users, poll time.Duration) {
	r.scenario("6. Analytics: отчёты (manager)")

	if u.managerToken == "" {
		r.skip("analytics", "нет токена manager")
		return
	}

	// Дадим Kafka-консьюмеру аналитики время поглотить события sales.completed.
	r.note("ждём 5s, чтобы Kafka-события долетели до ClickHouse")
	time.Sleep(5 * time.Second)

	// Sales report
	{
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.managerToken), 5*time.Second)
		resp, err := c.analytics.CreateSalesReport(ctx, &analyticspb.CreateSalesReportRequest{Period: analyticspb.ReportPeriod_MONTH})
		cancel()
		if r.record("create sales report", err, fmt.Sprintf("id=%s", short(resp.GetReportId()))) {
			pollSalesReport(r, c, u.managerToken, resp.GetReportId(), poll)
		}
	}

	// Forecast
	{
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.managerToken), 5*time.Second)
		resp, err := c.analytics.CreateForecast(ctx, &analyticspb.CreateForecastRequest{LookbackMonths: 1})
		cancel()
		if r.record("create forecast (1m)", err, fmt.Sprintf("id=%s", short(resp.GetReportId()))) {
			pollForecast(r, c, u.managerToken, resp.GetReportId(), poll)
		}
	}

	// Write-off report
	{
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.managerToken), 5*time.Second)
		resp, err := c.analytics.CreateWriteOffReport(ctx, &analyticspb.CreateWriteOffReportRequest{Period: analyticspb.ReportPeriod_MONTH})
		cancel()
		if r.record("create write-off report", err, fmt.Sprintf("id=%s", short(resp.GetReportId()))) {
			pollWriteOffReport(r, c, u.managerToken, resp.GetReportId(), poll)
		}
	}

	// Waste analysis
	{
		ctx, cancel := context.WithTimeout(withToken(context.Background(), u.managerToken), 5*time.Second)
		resp, err := c.analytics.CreateWasteAnalysis(ctx, &analyticspb.CreateWasteAnalysisRequest{Period: analyticspb.ReportPeriod_MONTH})
		cancel()
		if r.record("create waste analysis", err, fmt.Sprintf("id=%s", short(resp.GetReportId()))) {
			pollWasteAnalysis(r, c, u.managerToken, resp.GetReportId(), poll)
		}
	}
}

func pollSalesReport(r *runner, c *clients, token, id string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		ctx, cancel := context.WithTimeout(withToken(context.Background(), token), 5*time.Second)
		resp, err := c.analytics.GetSalesReport(ctx, &analyticspb.GetReportRequest{ReportId: id})
		cancel()
		if err != nil {
			r.record("get sales report", err, "")
			return
		}
		switch resp.GetStatus() {
		case analyticspb.ReportStatus_READY:
			r.record("sales report ready", nil,
				fmt.Sprintf("items=%d total_revenue=%.2f", len(resp.GetItems()), resp.GetTotalRevenue()))
			return
		case analyticspb.ReportStatus_FAILED:
			r.record("sales report ready", fmt.Errorf("status=FAILED: %s", resp.GetError()), "")
			return
		}
		if time.Now().After(deadline) {
			r.record("sales report ready", fmt.Errorf("timeout, status=%s", resp.GetStatus()), "")
			return
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

func pollForecast(r *runner, c *clients, token, id string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		ctx, cancel := context.WithTimeout(withToken(context.Background(), token), 5*time.Second)
		resp, err := c.analytics.GetForecast(ctx, &analyticspb.GetReportRequest{ReportId: id})
		cancel()
		if err != nil {
			r.record("get forecast", err, "")
			return
		}
		switch resp.GetStatus() {
		case analyticspb.ReportStatus_READY:
			r.record("forecast ready", nil,
				fmt.Sprintf("items=%d lookback=%dm", len(resp.GetItems()), resp.GetLookbackMonths()))
			return
		case analyticspb.ReportStatus_FAILED:
			r.record("forecast ready", fmt.Errorf("status=FAILED: %s", resp.GetError()), "")
			return
		}
		if time.Now().After(deadline) {
			r.record("forecast ready", fmt.Errorf("timeout, status=%s", resp.GetStatus()), "")
			return
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

func pollWriteOffReport(r *runner, c *clients, token, id string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		ctx, cancel := context.WithTimeout(withToken(context.Background(), token), 5*time.Second)
		resp, err := c.analytics.GetWriteOffReport(ctx, &analyticspb.GetReportRequest{ReportId: id})
		cancel()
		if err != nil {
			r.record("get write-off report", err, "")
			return
		}
		switch resp.GetStatus() {
		case analyticspb.ReportStatus_READY:
			r.record("write-off report ready", nil,
				fmt.Sprintf("items=%d total_qty=%d", len(resp.GetItems()), resp.GetTotalWrittenOff()))
			return
		case analyticspb.ReportStatus_FAILED:
			r.record("write-off report ready", fmt.Errorf("status=FAILED: %s", resp.GetError()), "")
			return
		}
		if time.Now().After(deadline) {
			r.record("write-off report ready", fmt.Errorf("timeout, status=%s", resp.GetStatus()), "")
			return
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

func pollWasteAnalysis(r *runner, c *clients, token, id string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		ctx, cancel := context.WithTimeout(withToken(context.Background(), token), 5*time.Second)
		resp, err := c.analytics.GetWasteAnalysis(ctx, &analyticspb.GetReportRequest{ReportId: id})
		cancel()
		if err != nil {
			r.record("get waste analysis", err, "")
			return
		}
		switch resp.GetStatus() {
		case analyticspb.ReportStatus_READY:
			r.record("waste analysis ready", nil, fmt.Sprintf("items=%d", len(resp.GetItems())))
			return
		case analyticspb.ReportStatus_FAILED:
			r.record("waste analysis ready", fmt.Errorf("status=FAILED: %s", resp.GetError()), "")
			return
		}
		if time.Now().After(deadline) {
			r.record("waste analysis ready", fmt.Errorf("timeout, status=%s", resp.GetStatus()), "")
			return
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

// 7. Logout ───────────────────────────────────────────────────────────────────

func scenarioLogout(r *runner, c *clients, u users) {
	r.scenario("7. Auth: logout")

	if u.pharmacistToken != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := c.auth.Logout(ctx, &authpb.LogoutRequest{Token: u.pharmacistToken})
		cancel()
		r.record("logout pharmacist", err, "")

		// Token больше не должен валидироваться.
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		_, verr := c.auth.ValidateToken(ctx2, &authpb.ValidateTokenRequest{Token: u.pharmacistToken})
		cancel2()
		if verr == nil {
			r.record("validate after logout", errors.New("token всё ещё валиден после logout"), "")
		} else {
			r.record("validate after logout", nil, "ожидаемо отклонён")
		}
	}
}

// short — короткое представление длинных id для красивого вывода.
func short(id string) string {
	if len(id) <= 12 {
		return id
	}
	return strings.ToLower(id[:8])
}
