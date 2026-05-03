package grpc

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"pharmacy/analytics/domain"
	pb "pharmacy/analytics/gen/analytics"
)

// Handler implements pb.AnalyticsServiceServer.
type Handler struct {
	pb.UnimplementedAnalyticsServiceServer
	uc AnalyticsUC
}

// NewHandler creates a new gRPC Handler.
func NewHandler(uc AnalyticsUC) *Handler {
	return &Handler{uc: uc}
}

// — Create methods —

func (h *Handler) CreateSalesReport(ctx context.Context, req *pb.CreateSalesReportRequest) (*pb.CreateReportResponse, error) {
	period := periodProtoToString(req.Period)
	id, err := h.uc.CreateSalesReport(ctx, period)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CreateReportResponse{ReportId: id}, nil
}

func (h *Handler) CreateWriteOffReport(ctx context.Context, req *pb.CreateWriteOffReportRequest) (*pb.CreateReportResponse, error) {
	period := periodProtoToString(req.Period)
	id, err := h.uc.CreateWriteOffReport(ctx, period)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CreateReportResponse{ReportId: id}, nil
}

func (h *Handler) CreateForecast(ctx context.Context, req *pb.CreateForecastRequest) (*pb.CreateReportResponse, error) {
	lookback := int(req.LookbackMonths)
	if lookback <= 0 {
		lookback = 1
	}
	id, err := h.uc.CreateForecast(ctx, lookback)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CreateReportResponse{ReportId: id}, nil
}

func (h *Handler) CreateWasteAnalysis(ctx context.Context, req *pb.CreateWasteAnalysisRequest) (*pb.CreateReportResponse, error) {
	period := periodProtoToString(req.Period)
	id, err := h.uc.CreateWasteAnalysis(ctx, period)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CreateReportResponse{ReportId: id}, nil
}

// — Get methods —

func (h *Handler) GetSalesReport(ctx context.Context, req *pb.GetReportRequest) (*pb.SalesReportResponse, error) {
	report, err := h.uc.GetSalesReport(ctx, req.ReportId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if report == nil {
		return nil, status.Error(codes.NotFound, "report not found")
	}

	resp := &pb.SalesReportResponse{
		Status:    domainStatusToProto(report.Status),
		CreatedAt: timestamppb.New(report.CreatedAt),
		Error:     report.Error,
	}
	if report.CompletedAt != nil {
		resp.CompletedAt = timestamppb.New(*report.CompletedAt)
	}

	if report.Status == domain.StatusReady && report.Result != nil {
		result := extractSalesResult(report.Result)
		if result != nil {
			resp.TotalRevenue = result.TotalRevenue
			resp.TotalQuantity = int32(result.TotalQuantity)
			for _, item := range result.Items {
				resp.Items = append(resp.Items, &pb.SalesReportItem{
					ProductId:    item.ProductID,
					ProductName:  item.ProductName,
					QuantitySold: int32(item.QtySold),
					Revenue:      item.Revenue,
					AvgPrice:     item.AvgPrice,
				})
			}
		}
	}
	return resp, nil
}

func (h *Handler) GetWriteOffReport(ctx context.Context, req *pb.GetReportRequest) (*pb.WriteOffReportResponse, error) {
	report, err := h.uc.GetWriteOffReport(ctx, req.ReportId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if report == nil {
		return nil, status.Error(codes.NotFound, "report not found")
	}

	resp := &pb.WriteOffReportResponse{
		Status:    domainStatusToProto(report.Status),
		CreatedAt: timestamppb.New(report.CreatedAt),
		Error:     report.Error,
	}
	if report.CompletedAt != nil {
		resp.CompletedAt = timestamppb.New(*report.CompletedAt)
	}

	if report.Status == domain.StatusReady && report.Result != nil {
		result := extractWriteOffResult(report.Result)
		if result != nil {
			resp.TotalLoss = result.TotalLoss
			resp.TotalWrittenOff = int32(result.TotalWrittenOff)
			for _, item := range result.Items {
				resp.Items = append(resp.Items, &pb.WriteOffReportItem{
					ProductId:          item.ProductID,
					ProductName:        item.ProductName,
					QuantityWrittenOff: int32(item.QtyWrittenOff),
					LossAmount:         item.LossAmount,
				})
			}
		}
	}
	return resp, nil
}

func (h *Handler) GetForecast(ctx context.Context, req *pb.GetReportRequest) (*pb.ForecastResponse, error) {
	report, err := h.uc.GetForecast(ctx, req.ReportId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if report == nil {
		return nil, status.Error(codes.NotFound, "report not found")
	}

	resp := &pb.ForecastResponse{
		Status:    domainStatusToProto(report.Status),
		CreatedAt: timestamppb.New(report.CreatedAt),
		Error:     report.Error,
	}
	if report.CompletedAt != nil {
		resp.CompletedAt = timestamppb.New(*report.CompletedAt)
	}

	if report.Status == domain.StatusReady && report.Result != nil {
		result := extractForecastResult(report.Result)
		if result != nil {
			resp.Period = result.Period
			resp.LookbackMonths = int32(result.LookbackMonths)
			for _, item := range result.Items {
				resp.Items = append(resp.Items, &pb.ForecastItem{
					ProductId:         item.ProductID,
					ProductName:       item.ProductName,
					TherapeuticGroup:  item.TherapeuticGroup,
					ForecastDemand:    int32(item.ForecastDemand),
					CurrentStock:      int32(item.CurrentStock),
					ExpiringNextMonth: int32(item.ExpiringNextMonth),
					UsableStock:       int32(item.UsableStock),
					WasteRatio:        item.WasteRatio,
					RecommendedOrder:  int32(item.RecommendedOrder),
					Confidence:        item.Confidence,
					Note:              item.Note,
				})
			}
		}
	}
	return resp, nil
}

func (h *Handler) GetWasteAnalysis(ctx context.Context, req *pb.GetReportRequest) (*pb.WasteAnalysisResponse, error) {
	report, err := h.uc.GetWasteAnalysis(ctx, req.ReportId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if report == nil {
		return nil, status.Error(codes.NotFound, "report not found")
	}

	resp := &pb.WasteAnalysisResponse{
		Status:    domainStatusToProto(report.Status),
		CreatedAt: timestamppb.New(report.CreatedAt),
		Error:     report.Error,
	}
	if report.CompletedAt != nil {
		resp.CompletedAt = timestamppb.New(*report.CompletedAt)
	}

	if report.Status == domain.StatusReady && report.Result != nil {
		result := extractWasteResult(report.Result)
		if result != nil {
			for _, item := range result.Items {
				resp.Items = append(resp.Items, &pb.WasteAnalysisItem{
					ProductId:        item.ProductID,
					ProductName:      item.ProductName,
					TherapeuticGroup: item.TherapeuticGroup,
					Received:         int32(item.Received),
					WrittenOff:       int32(item.WrittenOff),
					WasteRatio:       item.WasteRatio,
					LossAmount:       item.LossAmount,
					Recommendation:   item.Recommendation,
				})
			}
		}
	}
	return resp, nil
}

// — Mappers —

func periodProtoToString(p pb.ReportPeriod) string {
	switch p {
	case pb.ReportPeriod_MONTH:
		return "MONTH"
	case pb.ReportPeriod_HALF_YEAR:
		return "HALF_YEAR"
	case pb.ReportPeriod_YEAR:
		return "YEAR"
	default:
		return "MONTH"
	}
}

func domainStatusToProto(s domain.ReportStatus) pb.ReportStatus {
	switch s {
	case domain.StatusPending:
		return pb.ReportStatus_PENDING
	case domain.StatusProcessing:
		return pb.ReportStatus_PROCESSING
	case domain.StatusReady:
		return pb.ReportStatus_READY
	case domain.StatusFailed:
		return pb.ReportStatus_FAILED
	default:
		return pb.ReportStatus_REPORT_STATUS_UNSPECIFIED
	}
}

// extractSalesResult safely casts or re-unmarshals result to *domain.SalesReportResult.
func extractSalesResult(raw interface{}) *domain.SalesReportResult {
	if r, ok := raw.(*domain.SalesReportResult); ok {
		return r
	}
	// Fallback: re-marshal/unmarshal via JSON (when result is map[string]interface{})
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var r domain.SalesReportResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil
	}
	return &r
}

func extractWriteOffResult(raw interface{}) *domain.WriteOffReportResult {
	if r, ok := raw.(*domain.WriteOffReportResult); ok {
		return r
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var r domain.WriteOffReportResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil
	}
	return &r
}

func extractForecastResult(raw interface{}) *domain.ForecastResult {
	if r, ok := raw.(*domain.ForecastResult); ok {
		return r
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var r domain.ForecastResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil
	}
	return &r
}

func extractWasteResult(raw interface{}) *domain.WasteAnalysisResult {
	if r, ok := raw.(*domain.WasteAnalysisResult); ok {
		return r
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var r domain.WasteAnalysisResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil
	}
	return &r
}
