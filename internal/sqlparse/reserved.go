package sqlparse

import "strings"

// IsReserved reports whether a bare word is a reserved word in both MySQL and
// MariaDB.
//
// Only words reserved in both are listed. A reserved word cannot be a bare
// table name, so the formatter may upper-case it; a word reserved in only one
// of the two can be a table name on the other, and table names are case
// sensitive on a case-sensitive file system.
func IsReserved(word string) bool {
	return reserved[strings.ToUpper(word)]
}

// MakesAName reports whether a word right after tok is a name whatever it
// spells: after a dot it is a table or column, after @ a variable. Read as a
// keyword it would invent a clause the statement does not have.
func MakesAName(tok Token) bool {
	return tok.Kind == Punct && (tok.Text == "." || tok.Text == "@")
}

var reserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`
		ADD ALL ALTER AND AS ASC BETWEEN BIGINT BINARY BY CALL CASE CAST CHANGE
		CHAR CHARACTER CHECK COLLATE COLUMN CONSTRAINT CONVERT CREATE CROSS
		DATABASE DECIMAL DECLARE DEFAULT DELETE DESC DESCRIBE DISTINCT DIV DOUBLE
		DROP ELSE ESCAPE EXISTS EXPLAIN FALSE FLOAT FOR FORCE FOREIGN FROM GRANT
		GROUP HAVING IF IGNORE IN INDEX INNER INSERT INT INTEGER INTERVAL INTO IS
		JOIN KEY KEYS LEFT LIKE LIMIT LOCK LOOP MOD NATURAL NOT NULL ON OR ORDER
		OUTER PRIMARY PROCEDURE READ REFERENCES REGEXP RENAME REPLACE RETURN
		REVOKE RIGHT RLIKE SCHEMA SELECT SEPARATOR SET SHOW SMALLINT
		STRAIGHT_JOIN TABLE THEN TINYINT TO TRIGGER TRUE UNION UNIQUE UNSIGNED
		UPDATE USE USING VALUES VARCHAR WHEN WHERE WHILE WITH WRITE XOR ZEROFILL
	`) {
		reserved[w] = true
	}
}
