-- Detaching drops the rows in those partitions. Only run this if 2027 data is
-- genuinely disposable.
DROP TABLE IF EXISTS notifications_2027_q1, notifications_2027_q2,
                     notifications_2027_q3, notifications_2027_q4;
DROP TABLE IF EXISTS audit_logs_2027_01, audit_logs_2027_02, audit_logs_2027_03,
                     audit_logs_2027_04, audit_logs_2027_05, audit_logs_2027_06,
                     audit_logs_2027_07, audit_logs_2027_08, audit_logs_2027_09,
                     audit_logs_2027_10, audit_logs_2027_11, audit_logs_2027_12;
