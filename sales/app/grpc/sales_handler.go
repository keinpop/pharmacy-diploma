package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"pharmacy/sales/domain"
	usecase "pharmacy/sales/domain/use_case"
	pb "pharmacy/sales/gen/sales"
)

type Handler struct {
	pb.UnimplementedSalesServiceServer
	uc SalesUC
}

func NewHandler(uc SalesUC) *Handler { return &Handler{uc: uc} }

func (h *Handler) CreateSale(ctx context.Context, req *pb.CreateSaleRequest) (*pb.CreateSaleResponse, error) {
	if len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "items must not be empty")
	}

	items := make([]usecase.CreateSaleItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, usecase.CreateSaleItemInput{
			ProductID: item.ProductId,
			Quantity:  int(item.Quantity),
		})
	}

	sale, err := h.uc.CreateSale(ctx, usecase.CreateSaleInput{Items: items})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CreateSaleResponse{Sale: saleToPB(sale)}, nil
}

func (h *Handler) GetSale(ctx context.Context, req *pb.GetSaleRequest) (*pb.GetSaleResponse, error) {
	sale, err := h.uc.GetSale(ctx, req.Id)
	if err != nil {
		if err == domain.ErrSaleNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.GetSaleResponse{Sale: saleToPB(sale)}, nil
}

func (h *Handler) ListSales(ctx context.Context, req *pb.ListSalesRequest) (*pb.ListSalesResponse, error) {
	sales, total, err := h.uc.ListSales(ctx, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pbSales := make([]*pb.Sale, 0, len(sales))
	for _, s := range sales {
		pbSales = append(pbSales, saleToPB(s))
	}
	return &pb.ListSalesResponse{Sales: pbSales, Total: int32(total)}, nil
}

// — mappers —

func saleToPB(s *domain.Sale) *pb.Sale {
	items := make([]*pb.SaleItem, 0, len(s.Items))
	for _, item := range s.Items {
		items = append(items, &pb.SaleItem{
			Id:           item.ID,
			SaleId:       item.SaleID,
			ProductId:    item.ProductID,
			Quantity:     int32(item.Quantity),
			PricePerUnit: item.PricePerUnit,
			TotalPrice:   item.TotalPrice,
		})
	}
	return &pb.Sale{
		Id:          s.ID,
		Items:       items,
		TotalAmount: s.TotalAmount,
		SoldAt:      timestamppb.New(s.SoldAt),
	}
}
