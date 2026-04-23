package payment

import (
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

type PaymentRequest struct {
	OrderId string `protobuf:"bytes,1,opt,name=order_id,json=orderId,proto3" json:"order_id,omitempty"`
	Amount  int64  `protobuf:"varint,2,opt,name=amount,proto3" json:"amount,omitempty"`
}

func (x *PaymentRequest) Reset()         {}
func (x *PaymentRequest) String() string  { return x.OrderId }
func (x *PaymentRequest) ProtoMessage()  {}

func (x *PaymentRequest) GetOrderId() string {
	if x != nil {
		return x.OrderId
	}
	return ""
}

func (x *PaymentRequest) GetAmount() int64 {
	if x != nil {
		return x.Amount
	}
	return 0
}

type PaymentResponse struct {
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	OrderId       string                 `protobuf:"bytes,2,opt,name=order_id,json=orderId,proto3" json:"order_id,omitempty"`
	TransactionId string                 `protobuf:"bytes,3,opt,name=transaction_id,json=transactionId,proto3" json:"transaction_id,omitempty"`
	Amount        int64                  `protobuf:"varint,4,opt,name=amount,proto3" json:"amount,omitempty"`
	Status        string                 `protobuf:"bytes,5,opt,name=status,proto3" json:"status,omitempty"`
	CreatedAt     *timestamppb.Timestamp `protobuf:"bytes,6,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
}

func (x *PaymentResponse) Reset()         {}
func (x *PaymentResponse) String() string  { return x.Id }
func (x *PaymentResponse) ProtoMessage()  {}

func (x *PaymentResponse) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *PaymentResponse) GetOrderId() string {
	if x != nil {
		return x.OrderId
	}
	return ""
}

func (x *PaymentResponse) GetTransactionId() string {
	if x != nil {
		return x.TransactionId
	}
	return ""
}

func (x *PaymentResponse) GetAmount() int64 {
	if x != nil {
		return x.Amount
	}
	return 0
}

func (x *PaymentResponse) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *PaymentResponse) GetCreatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.CreatedAt
	}
	return nil
}
