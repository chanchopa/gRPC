
CREATE TABLE IF NOT EXISTS payments (
    id             VARCHAR(36)  PRIMARY KEY,        -- UUID
    order_id       VARCHAR(36)  NOT NULL,
    transaction_id VARCHAR(36),                     -- NULL when Declined
    amount         BIGINT       NOT NULL CHECK (amount > 0),
    status         VARCHAR(20)  NOT NULL
                       CHECK (status IN ('Authorized', 'Declined')),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_order_id ON payments (order_id);
