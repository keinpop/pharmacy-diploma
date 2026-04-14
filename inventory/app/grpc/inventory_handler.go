package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"pharmacy/inventory/domain"
	usecase "pharmacy/inventory/domain/use_case"
	pb "pharmacy/inventory/gen/inventory"
)

type Handler struct {
	pb.UnimplementedInventoryServiceServer
	uc InventoryUC
}

func NewHandler(uc InventoryUC) *Handler { return &Handler{uc: uc} }

// Product

func (h *Handler) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	p, err := h.uc.CreateProduct(ctx, usecase.CreateProductInput{
		Name: req.Name, TradeName: req.TradeName, ActiveSubstance: req.ActiveSubstance,
		Form: req.Form, Dosage: req.Dosage, Category: domain.Category(req.Category),
		StorageConditions: req.StorageConditions, Unit: req.Unit, ReorderPoint: int(req.ReorderPoint),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CreateProductResponse{Product: productToPB(p)}, nil
}

func (h *Handler) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.ProductResponse, error) {
	p, err := h.uc.GetProduct(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &pb.ProductResponse{Product: productToPB(p)}, nil
}

func (h *Handler) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	products, total, err := h.uc.ListProducts(ctx, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.ListProductsResponse{Products: productsToPB(products), Total: int32(total)}, nil
}

func (h *Handler) SearchProducts(ctx context.Context, req *pb.SearchProductsRequest) (*pb.ListProductsResponse, error) {
	products, err := h.uc.SearchProducts(ctx, req.Query, int(req.Limit))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.ListProductsResponse{Products: productsToPB(products)}, nil
}

func (h *Handler) GetAnalogs(ctx context.Context, req *pb.GetAnalogsRequest) (*pb.ListProductsResponse, error) {
	products, err := h.uc.GetAnalogs(ctx, req.ProductId, int(req.Limit))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.ListProductsResponse{Products: productsToPB(products)}, nil
}

// Inventory

func (h *Handler) ReceiveBatch(ctx context.Context, req *pb.ReceiveBatchRequest) (*pb.BatchResponse, error) {
	b, err := h.uc.ReceiveBatch(ctx, usecase.ReceiveBatchInput{
		ProductID:     req.ProductId,
		SeriesNumber:  req.SeriesNumber,
		ExpiresAt:     req.ExpiresAt.AsTime(),
		Quantity:      int(req.Quantity),
		PurchasePrice: req.PurchasePrice,
		RetailPrice:   req.RetailPrice,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.BatchResponse{Batch: batchToPB(b)}, nil
}

func (h *Handler) GetStock(ctx context.Context, req *pb.GetStockRequest) (*pb.GetStockResponse, error) {
	s, err := h.uc.GetStock(ctx, req.ProductId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &pb.GetStockResponse{Stock: stockToPB(s)}, nil
}

func (h *Handler) DeductStock(ctx context.Context, req *pb.DeductStockRequest) (*pb.DeductStockResponse, error) {
	batchID, retailPrice, err := h.uc.DeductStock(ctx, req.ProductId, int(req.Quantity), req.OrderId)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pb.DeductStockResponse{Success: true, BatchId: batchID, RetailPrice: retailPrice}, nil
}

func (h *Handler) ListExpiringBatches(ctx context.Context, req *pb.ListExpiringRequest) (*pb.ListExpiringResponse, error) {
	batches, err := h.uc.ListExpiringBatches(ctx, int(req.DaysAhead))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.ListExpiringResponse{Batches: batchesToPB(batches)}, nil
}

func (h *Handler) ListLowStock(ctx context.Context, _ *pb.ListLowStockRequest) (*pb.ListLowStockResponse, error) {
	items, err := h.uc.ListLowStock(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	pbItems := make([]*pb.StockItem, 0, len(items))
	for _, s := range items {
		pbItems = append(pbItems, stockToPB(s))
	}
	return &pb.ListLowStockResponse{Items: pbItems}, nil
}

func (h *Handler) WriteOffExpired(ctx context.Context, _ *pb.WriteOffExpiredRequest) (*pb.WriteOffExpiredResponse, error) {
	count, err := h.uc.WriteOffExpired(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.WriteOffExpiredResponse{WrittenOffCount: int32(count)}, nil
}

// mappers ─

func productToPB(p *domain.Product) *pb.Product {
	return &pb.Product{
		Id: p.ID, Name: p.Name, TradeName: p.TradeName, ActiveSubstance: p.ActiveSubstance,
		Form: p.Form, Dosage: p.Dosage, Category: string(p.Category),
		StorageConditions: p.StorageConditions, Unit: p.Unit, ReorderPoint: int32(p.ReorderPoint),
		CreatedAt: timestamppb.New(p.CreatedAt),
		UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
}

func productsToPB(products []*domain.Product) []*pb.Product {
	out := make([]*pb.Product, 0, len(products))
	for _, p := range products {
		out = append(out, productToPB(p))
	}
	return out
}

var batchStatusMap = map[domain.BatchStatus]pb.BatchStatus{
	domain.BatchStatusAvailable:    pb.BatchStatus_AVAILABLE,
	domain.BatchStatusExpiringSoon: pb.BatchStatus_EXPIRING_SOON,
	domain.BatchStatusExpired:      pb.BatchStatus_EXPIRED,
	domain.BatchStatusWrittenOff:   pb.BatchStatus_WRITTEN_OFF,
}

func batchToPB(b *domain.Batch) *pb.Batch {
	st := batchStatusMap[b.Status]
	return &pb.Batch{
		Id: b.ID, ProductId: b.ProductID, SeriesNumber: b.SeriesNumber,
		ExpiresAt: timestamppb.New(b.ExpiresAt), ReceivedAt: timestamppb.New(b.ReceivedAt),
		Quantity: int32(b.Quantity), Reserved: int32(b.Reserved),
		PurchasePrice: b.PurchasePrice, RetailPrice: b.RetailPrice, Status: st,
	}
}

func batchesToPB(batches []*domain.Batch) []*pb.Batch {
	out := make([]*pb.Batch, 0, len(batches))
	for _, b := range batches {
		out = append(out, batchToPB(b))
	}
	return out
}

func stockToPB(s *domain.StockItem) *pb.StockItem {
	return &pb.StockItem{
		ProductId:     s.ProductID,
		TotalQuantity: int32(s.TotalQuantity),
		Reserved:      int32(s.Reserved),
		Available:     int32(s.Available()),
		LowStock:      s.LowStock(),
	}
}

// keep time import
var _ = time.Now
