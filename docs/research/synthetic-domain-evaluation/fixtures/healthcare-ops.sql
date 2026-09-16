-- Synthetic healthcare operations fixture. All identifiers and records are fictional.
CREATE DATABASE IF NOT EXISTS healthcare_ops CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE healthcare_ops;

CREATE TABLE clinics (
  id BIGINT PRIMARY KEY,
  slug VARCHAR(80) NOT NULL UNIQUE,
  display_name VARCHAR(120) NOT NULL,
  timezone_name VARCHAR(64) NOT NULL
);

CREATE TABLE appointments (
  id BIGINT PRIMARY KEY,
  clinic_id BIGINT NOT NULL,
  public_id VARCHAR(32) NOT NULL UNIQUE,
  status VARCHAR(24) NOT NULL,
  scheduled_local DATETIME NOT NULL,
  scheduled_utc DATETIME NOT NULL,
  completed_at DATETIME NULL,
  note VARCHAR(200) NULL,
  CONSTRAINT fk_appointments_clinic FOREIGN KEY (clinic_id) REFERENCES clinics(id)
);

CREATE TABLE appointment_events (
  id BIGINT PRIMARY KEY,
  appointment_id BIGINT NOT NULL,
  event_type VARCHAR(40) NOT NULL,
  occurred_at DATETIME NOT NULL,
  message VARCHAR(200) NULL,
  CONSTRAINT fk_events_appointment FOREIGN KEY (appointment_id) REFERENCES appointments(id)
);

CREATE TABLE scheduler_job_runs (
  id BIGINT PRIMARY KEY,
  job_key VARCHAR(32) NOT NULL UNIQUE,
  status VARCHAR(24) NOT NULL,
  started_at DATETIME NOT NULL,
  finished_at DATETIME NULL,
  error_code VARCHAR(48) NULL,
  summary VARCHAR(200) NULL
);

CREATE TABLE access_audit_events (
  id BIGINT PRIMARY KEY,
  actor_ref VARCHAR(40) NOT NULL,
  action VARCHAR(40) NOT NULL,
  object_type VARCHAR(40) NOT NULL,
  occurred_at DATETIME NOT NULL,
  details TEXT NULL
);

INSERT INTO clinics VALUES
  (1, 'central-care', 'Central Care · 서울', 'America/New_York'),
  (2, 'central-care-east', 'Central Care East', 'America/New_York'),
  (3, 'river-clinic', 'River Clinic', 'UTC');

INSERT INTO appointments VALUES
  (1,1,'APT-3001','scheduled','2026-03-08 01:30:00','2026-03-08 06:30:00',NULL,'DST 이전'),
  (2,1,'APT-3002','cancelled','2026-03-08 03:30:00','2026-03-08 07:30:00',NULL,'DST 이후'),
  (3,2,'APT-3003','completed','2026-03-08 04:00:00','2026-03-08 08:00:00','2026-03-08 08:42:00','follow-up ✓'),
  (4,2,'APT-3004','scheduled','2026-03-09 09:00:00','2026-03-09 13:00:00',NULL,NULL),
  (5,1,'APT-3005','completed','2026-03-09 10:00:00','2026-03-09 14:00:00','2026-03-09 14:31:00','정상 완료'),
  (6,3,'APT-3006','scheduled','2026-03-10 08:00:00','2026-03-10 08:00:00',NULL,NULL),
  (7,1,'APT-3007','no_show','2026-03-10 11:30:00','2026-03-10 15:30:00',NULL,'pseudonymous record'),
  (8,2,'APT-3008','completed','2026-03-10 12:30:00','2026-03-10 16:30:00','2026-03-10 17:01:00',NULL),
  (9,3,'APT-3009','cancelled','2026-03-11 08:30:00','2026-03-11 08:30:00',NULL,NULL),
  (10,1,'APT-3010','scheduled','2026-03-11 14:00:00','2026-03-11 18:00:00',NULL,NULL),
  (11,2,'APT-3011','scheduled','2026-03-11 14:30:00','2026-03-11 18:30:00',NULL,NULL),
  (12,3,'APT-3012','completed','2026-03-12 09:00:00','2026-03-12 09:00:00','2026-03-12 09:25:00','검토 완료'),
  (13,1,'APT-3013','scheduled','2026-03-12 15:00:00','2026-03-12 19:00:00',NULL,NULL),
  (14,2,'APT-3014','cancelled','2026-03-12 16:00:00','2026-03-12 20:00:00',NULL,NULL),
  (15,3,'APT-3015','scheduled','2026-03-13 10:00:00','2026-03-13 10:00:00',NULL,'UTF-8 café');

INSERT INTO appointment_events VALUES
  (1,1,'created','2026-03-01 10:00:00','appointment scheduled'),
  (2,2,'cancelled','2026-03-07 22:00:00','cancelled before DST boundary'),
  (3,3,'completed','2026-03-08 08:42:00','완료'),
  (4,5,'completed','2026-03-09 14:31:00','completed normally');

INSERT INTO scheduler_job_runs VALUES
  (1,'JOB-2041','succeeded','2026-03-08 06:00:00','2026-03-08 06:01:12',NULL,'generated schedules'),
  (2,'JOB-2042','failed','2026-03-08 07:00:00','2026-03-08 07:00:19','DST_WINDOW_COLLISION','central-care transition conflict'),
  (3,'JOB-2043','succeeded','2026-03-08 08:00:00','2026-03-08 08:01:03',NULL,'recovery run');

INSERT INTO access_audit_events VALUES
  (1,'ops-ada','view','appointment','2026-03-08 06:35:00','routine synthetic access'),
  (2,'job-scheduler','update','schedule','2026-03-08 07:00:19','failure recorded'),
  (3,'ops-min','view','job_run','2026-03-08 07:05:00','investigated JOB-2042'),
  (4,'ops-ada','export','job_run','2026-03-08 07:10:00','approved synthetic export'),
  (5,'audit-bot','scan','access_log','2026-03-08 07:15:00','canary=NOT_A_REAL_TOKEN_2042'),
  (6,'ops-min','view','clinic','2026-03-08 07:20:00','checked central-care-east');
