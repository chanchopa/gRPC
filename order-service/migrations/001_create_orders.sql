CREATE TABLE IF NOT EXISTS orders (
    id               VARCHAR(36)   PRIMARY KEY,          -- UUID
    customer_id      VARCHAR(36)   NOT NULL,
    item_name        VARCHAR(255)  NOT NULL,
    amount           BIGINT        NOT NULL CHECK (amount > 0), -- cents, int64, never float
    status           VARCHAR(20)   NOT NULL DEFAULT 'Pending'
                         CHECK (status IN ('Pending', 'Paid', 'Failed', 'Cancelled')),
    idempotency_key  VARCHAR(255)  UNIQUE,               -- Bonus: idempotency support
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_customer_id       ON orders (customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_status            ON orders (status);
CREATE INDEX IF NOT EXISTS idx_orders_idempotency_key   ON orders (idempotency_key);
