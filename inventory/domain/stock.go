package domain

type StockItem struct {
	ProductID     string
	TotalQuantity int // сумма Quantity по всем активным партиям
	Reserved      int // сумма Reserved
	ReorderPoint  int // копируется из Product
}

func (s *StockItem) Available() int { return s.TotalQuantity - s.Reserved }
func (s *StockItem) LowStock() bool { return s.Available() <= s.ReorderPoint }
