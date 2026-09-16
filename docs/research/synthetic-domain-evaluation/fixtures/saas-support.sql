-- Synthetic fixture for sde-20260915-005610 / saas-support.
-- All organizations and people are fictional.

DROP DATABASE IF EXISTS saas_support;
CREATE DATABASE saas_support CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE saas_support;

CREATE TABLE tenants (
  tenant_id BIGINT PRIMARY KEY,
  slug VARCHAR(80) NOT NULL UNIQUE,
  display_name VARCHAR(160) NOT NULL,
  lifecycle ENUM('trial','active','cancelled') NOT NULL,
  created_at DATETIME(6) NOT NULL
);

CREATE TABLE users (
  user_id BIGINT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  full_name VARCHAR(120) NOT NULL,
  email VARCHAR(180) NOT NULL,
  locale VARCHAR(16) NOT NULL,
  FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id),
  UNIQUE KEY uq_user_email_per_tenant (tenant_id, email)
);

CREATE TABLE subscriptions (
  subscription_id BIGINT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  plan_code VARCHAR(40) NOT NULL,
  status ENUM('trialing','active','past_due','cancelled') NOT NULL,
  seats INT NOT NULL,
  started_at DATETIME NOT NULL,
  cancelled_at DATETIME NULL,
  FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id)
);

CREATE TABLE feature_entitlements (
  entitlement_id BIGINT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  feature_key VARCHAR(80) NOT NULL,
  enabled BOOLEAN NOT NULL,
  source VARCHAR(40) NOT NULL,
  FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id),
  UNIQUE KEY uq_tenant_feature (tenant_id, feature_key)
);

CREATE TABLE support_tickets (
  ticket_id BIGINT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  requester_user_id BIGINT NOT NULL,
  priority ENUM('P1','P2','P3','P4') NOT NULL,
  status ENUM('open','pending','solved','closed') NOT NULL,
  subject VARCHAR(240) NOT NULL,
  customer_note TEXT NULL,
  created_at DATETIME(6) NOT NULL,
  resolved_at DATETIME(6) NULL,
  FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id),
  FOREIGN KEY (requester_user_id) REFERENCES users(user_id)
);

INSERT INTO tenants VALUES
  (101,'northstar-labs','Northstar Labs','active','2025-01-10 09:00:00.000000'),
  (102,'blue-harbor','Blue Harbor, Inc.','active','2025-02-11 09:00:00.000000'),
  (103,'hanul-data','한울 데이터','active','2025-03-12 09:00:00.000000'),
  (104,'acme-field','Acme Field','trial','2025-04-13 09:00:00.000000'),
  (105,'cedar-work','Cedar Work','active','2025-05-14 09:00:00.000000'),
  (106,'orbit-ops','Orbit Ops','cancelled','2025-06-15 09:00:00.000000'),
  (107,'lumen-co','Lumen Co','active','2025-07-16 09:00:00.000000'),
  (108,'pixel-yard','Pixel Yard','active','2025-08-17 09:00:00.000000'),
  (109,'riverbank','Riverbank','trial','2025-09-18 09:00:00.000000'),
  (110,'zenith-works','Zenith Works','active','2025-10-19 09:00:00.000000');

INSERT INTO users VALUES
  (1001,101,'Alex Kim','alex@northstar.example','en-US'),
  (1002,102,'Alex Kim','alex@blueharbor.example','en-US'),
  (1003,103,'김하늘','haneul@hanul.example','ko-KR'),
  (1004,104,'Maya Chen','maya@acme.example','en-US'),
  (1005,105,'Noah Park','noah@cedar.example','en-US'),
  (1006,106,'Iris Moon','iris@orbit.example','en-US'),
  (1007,107,'Ravi Shah','ravi@lumen.example','en-GB'),
  (1008,108,'Sara Ito','sara@pixel.example','ja-JP'),
  (1009,109,'Owen Lee','owen@riverbank.example','en-US'),
  (1010,110,'Ella Jones','ella@zenith.example','en-US');

INSERT INTO subscriptions VALUES
  (2001,101,'enterprise','active',80,'2025-01-10 09:00:00',NULL),
  (2002,102,'business','active',35,'2025-02-11 09:00:00',NULL),
  (2003,103,'business','past_due',20,'2025-03-12 09:00:00',NULL),
  (2004,104,'starter','trialing',5,'2025-04-13 09:00:00',NULL),
  (2005,105,'enterprise','active',120,'2025-05-14 09:00:00',NULL),
  (2006,106,'business','cancelled',18,'2025-06-15 09:00:00','2026-08-31 18:30:00'),
  (2007,107,'starter','active',8,'2025-07-16 09:00:00',NULL),
  (2008,108,'business','active',42,'2025-08-17 09:00:00',NULL),
  (2009,109,'starter','trialing',3,'2025-09-18 09:00:00',NULL),
  (2010,110,'enterprise','active',200,'2025-10-19 09:00:00',NULL);

INSERT INTO feature_entitlements VALUES
  (3001,101,'sso',1,'plan'),
  (3002,101,'audit_log',1,'plan'),
  (3003,102,'advanced_export',1,'plan'),
  (3004,103,'advanced_export',0,'override'),
  (3005,104,'basic_export',1,'plan'),
  (3006,105,'advanced_export',1,'plan'),
  (3007,106,'advanced_export',0,'cancelled'),
  (3008,107,'basic_export',1,'plan'),
  (3009,108,'advanced_export',1,'plan'),
  (3010,110,'advanced_export',1,'plan');

INSERT INTO support_tickets VALUES
  (9001,101,1001,'P1','open','Advanced export unavailable','Export page says “not entitled”.\nCustomer launch is today, please escalate.','2026-09-15 00:12:34.123456',NULL),
  (9002,102,1002,'P2','pending','CSV columns shifted','Company name contains a comma, result must stay quoted.','2026-09-14 10:20:30.000000',NULL),
  (9003,103,1003,'P1','open','로그인 지연','서울 리전에서 간헐적 지연','2026-09-15 00:45:00.500000',NULL),
  (9004,105,1005,'P3','solved','Seat count question',NULL,'2026-09-10 08:00:00.000000','2026-09-10 09:00:00.000000'),
  (9005,106,1006,'P2','closed','Cancellation confirmation','Cancelled as requested.','2026-08-31 18:00:00.000000','2026-08-31 18:31:00.000000'),
  (9006,108,1008,'P2','open','Webhook retries','Retries seen after 09:00 UTC.','2026-09-15 01:05:00.000000',NULL);

-- Planned query 1: explain Northstar's missing advanced_export entitlement.
-- Expected: one row; enterprise/active, entitlement row absent, effective_enabled=0.
SELECT t.slug, s.plan_code, s.status AS subscription_status,
       fe.feature_key, fe.enabled, COALESCE(fe.enabled, 0) AS effective_enabled
FROM tenants t
JOIN subscriptions s ON s.tenant_id = t.tenant_id
LEFT JOIN feature_entitlements fe
  ON fe.tenant_id = t.tenant_id AND fe.feature_key = 'advanced_export'
WHERE t.slug = 'northstar-labs';

-- Planned query 2 / escalation export: open tickets, sorted by priority and age.
-- Expected: 9001, 9003, 9006 in that order; duplicate names remain tenant-qualified.
SELECT st.ticket_id, t.slug AS tenant, u.full_name AS requester,
       st.priority, st.status, st.subject, st.customer_note, st.created_at
FROM support_tickets st
JOIN tenants t ON t.tenant_id = st.tenant_id
JOIN users u ON u.user_id = st.requester_user_id
WHERE st.status = 'open'
ORDER BY FIELD(st.priority, 'P1','P2','P3','P4'), st.created_at;

-- Planned query 3: subscription aggregation.
-- Expected: active=6/485 seats, cancelled=1/18, past_due=1/20, trialing=2/8.
SELECT status, COUNT(*) AS subscriptions, SUM(seats) AS seats
FROM subscriptions
GROUP BY status
ORDER BY status;

-- Controlled read-only write (execute only in dv; UI refusal expected; numeric error code unobserved):
-- DELETE FROM support_tickets WHERE ticket_id = 9001;
