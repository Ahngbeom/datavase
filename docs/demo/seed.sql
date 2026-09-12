CREATE DATABASE IF NOT EXISTS shop;
USE shop;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS customers;
CREATE TABLE customers (
  id INT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(40) NOT NULL,
  email VARCHAR(80) NOT NULL UNIQUE,
  created_at DATETIME NOT NULL
);
CREATE TABLE orders (
  id INT AUTO_INCREMENT PRIMARY KEY,
  customer VARCHAR(40) NOT NULL,
  total DECIMAL(10,2) NOT NULL,
  status ENUM('paid','shipped','refunded') NOT NULL,
  placed_at DATETIME NOT NULL,
  INDEX (status)
);
INSERT INTO customers (name, email, created_at) VALUES
  ('Ada Lovelace', 'ada@example.com',   '2026-08-02 09:14:00'),
  ('Grace Hopper', 'grace@example.com', '2026-08-05 17:40:00'),
  ('Linus T.',     'linus@example.com', '2026-08-11 12:03:00'),
  ('Ken Thompson', 'ken@example.com',   '2026-08-20 08:55:00');
INSERT INTO orders (customer, total, status, placed_at) VALUES
  ('Ada Lovelace', 129.00,  'shipped',  '2026-09-01 10:12:00'),
  ('Grace Hopper', 48.50,   'paid',     '2026-09-03 14:02:00'),
  ('Linus T.',     1024.00, 'paid',     '2026-09-04 09:30:00'),
  ('Ada Lovelace', 19.99,   'refunded', '2026-09-06 18:45:00'),
  ('Ken Thompson', 256.00,  'shipped',  '2026-09-08 11:11:00'),
  ('Grace Hopper', 73.25,   'paid',     '2026-09-10 16:20:00');
