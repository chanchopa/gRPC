package order

import (
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

type OrderRequest struct {
	OrderId string `protobuf:"bytes,1,opt,name=order_id,json=orderId,proto3" json:"order_id,omitempty"`
}

func (x *OrderRequest) Reset()         {}
func (x *OrderRequest) String() string  { return x.OrderId }
func (x *OrderRequest) ProtoMessage()  {}

func (x *OrderRequest) GetOrderId() string {
	if x != nil {
		return x.OrderId
	}
	return ""
}

type OrderStatusUpdate struct {
	OrderId   string                 `protobuf:"bytes,1,opt,name=order_id,json=orderId,proto3" json:"order_id,omitempty"`
	Status    string                 `protobuf:"bytes,2,opt,name=status,proto3" json:"status,omitempty"`
	UpdatedAt *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=updated_at,json=updatedAt,proto3" json:"updated_at,omitempty"`
}

func (x *OrderStatusUpdate) Reset()         {}
func (x *OrderStatusUpdate) String() string  { return x.OrderId }
func (x *OrderStatusUpdate) ProtoMessage()  {}

func (x *OrderStatusUpdate) GetOrderId() string {
	if x != nil {
		return x.OrderId
	}
	return ""
}

func (x *OrderStatusUpdate) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *OrderStatusUpdate) GetUpdatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.UpdatedAt
	}
	return nil
}
