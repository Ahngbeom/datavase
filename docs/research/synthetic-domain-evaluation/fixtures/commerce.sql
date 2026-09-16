-- Synthetic commerce incident-response fixture for run sde-20260915-005610.
-- All people, orders, payments, and events are fictional.

SET NAMES utf8mb4;

CREATE TABLE orders (
  id BIGINT UNSIGNED PRIMARY KEY,
  order_no VARCHAR(32) NOT NULL UNIQUE,
  customer_name VARCHAR(100) NOT NULL,
  status ENUM('PENDING','PAID','FULFILLING','SHIPPED','REFUNDED','CANCELLED') NOT NULL,
  total DECIMAL(10,2) NOT NULL,
  created_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE payment_attempts (
  id BIGINT UNSIGNED PRIMARY KEY,
  order_id BIGINT UNSIGNED NOT NULL,
  attempt_no INT UNSIGNED NOT NULL,
  status ENUM('AUTHORIZED','CAPTURED','DECLINED') NOT NULL,
  failure_code VARCHAR(40) NULL,
  amount DECIMAL(10,2) NOT NULL,
  attempted_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_payment_attempt (order_id, attempt_no),
  CONSTRAINT fk_payment_order FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE refunds (
  id BIGINT UNSIGNED PRIMARY KEY,
  order_id BIGINT UNSIGNED NOT NULL,
  amount DECIMAL(10,2) NOT NULL,
  reason VARCHAR(160) NOT NULL,
  refunded_at DATETIME(6) NOT NULL,
  CONSTRAINT fk_refund_order FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE fulfilment_events (
  id BIGINT UNSIGNED PRIMARY KEY,
  order_id BIGINT UNSIGNED NOT NULL,
  event_type ENUM('PICKED','PACKED','SHIPPED','DELIVERED') NOT NULL,
  occurred_at DATETIME(6) NULL,
  note VARCHAR(160) NULL,
  CONSTRAINT fk_fulfilment_order FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO orders (id, order_no, customer_name, status, total, created_at) VALUES
  (1,  'ORD-1001', 'Alice Example', 'SHIPPED',    120.00, '2026-09-14 08:00:00.000000'),
  (2,  'ORD-1002', '김하늘',        'PAID',        42.50, '2026-09-14 08:10:00.000000'),
  (3,  'ORD-1003', 'Renée Test',   'FULFILLING',  75.20, '2026-09-14 08:20:00.000000'),
  (4,  'ORD-1004', 'Mario Demo',   'PENDING',     15.00, '2026-09-14 08:30:00.000000'),
  (5,  'ORD-1005', '佐藤テスト',      'REFUNDED',    19.95, '2026-09-14 08:40:00.000000'),
  (6,  'ORD-1006', 'Nora Fixture', 'SHIPPED',     88.80, '2026-09-14 08:50:00.000000'),
  (7,  'ORD-1007', '박지민',        'PENDING',     64.00, '2026-09-14 09:00:00.000000'),
  (8,  'ORD-1008', 'Omar Sample',  'PAID',        31.25, '2026-09-14 09:10:00.000000'),
  (9,  'ORD-1009', 'Léa Fiction',  'CANCELLED',   52.10, '2026-09-14 09:20:00.000000'),
  (10, 'ORD-1010', 'Ana Sandbox',  'FULFILLING', 103.40, '2026-09-14 09:30:00.000000'),
  (11, 'ORD-1011', 'Иван Тест',    'PAID',        27.70, '2026-09-14 09:40:00.000000'),
  (12, 'ORD-1012', 'Zoë Mock',     'SHIPPED',    210.00, '2026-09-14 09:50:00.000000');

INSERT INTO payment_attempts (id, order_id, attempt_no, status, failure_code, amount, attempted_at) VALUES
  (101, 1,  1, 'CAPTURED', NULL,               120.00, '2026-09-14 08:01:00.000000'),
  (102, 2,  1, 'CAPTURED', NULL,                42.50, '2026-09-14 08:11:00.000000'),
  (103, 3,  1, 'CAPTURED', NULL,                75.20, '2026-09-14 08:21:00.000000'),
  (104, 4,  1, 'DECLINED', 'INSUFFICIENT_FUNDS',15.00, '2026-09-14 08:31:00.000000'),
  (105, 5,  1, 'CAPTURED', NULL,                19.95, '2026-09-14 08:41:00.000000'),
  (106, 6,  1, 'DECLINED', 'TIMEOUT',           88.80, '2026-09-14 08:51:00.000000'),
  (107, 6,  2, 'CAPTURED', NULL,                88.80, '2026-09-14 08:52:00.000000'),
  (108, 7,  1, 'AUTHORIZED', NULL,              64.00, '2026-09-14 09:01:00.000000'),
  (109, 7,  2, 'DECLINED', 'DO_NOT_HONOR',      64.00, '2026-09-14 09:02:00.000000'),
  (110, 8,  1, 'CAPTURED', NULL,                31.25, '2026-09-14 09:11:00.000000'),
  (111, 10, 1, 'CAPTURED', NULL,               103.40, '2026-09-14 09:31:00.000000'),
  (112, 11, 1, 'CAPTURED', NULL,                27.70, '2026-09-14 09:41:00.000000'),
  (113, 12, 1, 'CAPTURED', NULL,               210.00, '2026-09-14 09:51:00.000000');

INSERT INTO refunds (id, order_id, amount, reason, refunded_at) VALUES
  (201, 5, 19.95, 'duplicate order — synthetic', '2026-09-14 10:00:00.000000');

INSERT INTO fulfilment_events (id, order_id, event_type, occurred_at, note) VALUES
  (301, 1,  'SHIPPED',   '2026-09-14 12:00:00.000000', 'carrier accepted'),
  (302, 3,  'PACKED',    '2026-09-14 12:10:00.000000', '포장 완료'),
  (303, 7,  'PICKED',    NULL,                         'awaiting scan'),
  (304, 12, 'DELIVERED', '2026-09-15 07:30:00.000000', 'left at reception');

-- Expected result 1: ORD-1007 | PENDING | DECLINED | DO_NOT_HONOR
SELECT o.order_no, o.status, p.status AS latest_payment, p.failure_code
FROM orders o JOIN payment_attempts p ON p.order_id = o.id
WHERE o.order_no = 'ORD-1007'
ORDER BY p.attempted_at DESC LIMIT 1;

-- Expected result 2 in ENUM order: PENDING 2, PAID 3, FULFILLING 2,
-- SHIPPED 3, REFUNDED 1, CANCELLED 1.
SELECT status, COUNT(*) AS orders FROM orders GROUP BY status ORDER BY status;

-- Expected result 3: ORD-1005 | 19.95 | duplicate order — synthetic
SELECT o.order_no, r.amount, r.reason
FROM refunds r JOIN orders o ON o.id = r.order_id
WHERE r.amount = 19.95;

-- Rendering probe: ORD-1007 | 박지민 | NULL.
SELECT o.order_no, o.customer_name, f.occurred_at
FROM orders o JOIN fulfilment_events f ON f.order_id = o.id
WHERE o.order_no = 'ORD-1007';
