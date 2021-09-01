package rds

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rdsdataservice"
	"regexp"
	"strconv"
	"time"
)

var ordinalRegex = regexp.MustCompile("\\?{1}")

// DialectMySQL for version 5.7
type DialectMySQL struct {
}

// MigrateQuery converts a mysql query into an RDS stateement.
func (d *DialectMySQL) MigrateQuery(query string, args []driver.NamedValue) (*rdsdataservice.ExecuteStatementInput, error) {
	// Make sure we're not mixing and matching.
	ordinal := false
	named := false
	for _, arg := range args {
		if arg.Name != "" {
			named = true
		}
		if arg.Ordinal > 0 {
			ordinal = true
		}
		if named && ordinal {
			return nil, ErrNoMixedParams
		}
	}

	// If we're ordinal, convert to named
	if ordinal {
		namedArgs := make([]driver.NamedValue, len(args))
		for i, v := range args {
			namedArgs[i] = driver.NamedValue{
				Name:  strconv.Itoa(v.Ordinal),
				Value: v.Value,
			}
		}
		args = namedArgs

		idx := 0
		query = ordinalRegex.ReplaceAllStringFunc(query, func(s string) string {
			idx = idx + 1 // ordinal regex are one-indexed
			return fmt.Sprintf(":%d", idx)
		})

		params, err := ConvertNamedValues(namedArgs)
		return &rdsdataservice.ExecuteStatementInput{
			Parameters: params,
			Sql:        aws.String(query),
		}, err
	}

	params, err := ConvertNamedValues(args)
	return &rdsdataservice.ExecuteStatementInput{
		Parameters: params,
		Sql:        aws.String(query),
	}, err
}

// GetFieldConverter knows how to parse column results.s
func (d *DialectMySQL) GetFieldConverter(columnType string) FieldConverter {
	switch columnType {
	case "TINYINT UNSIGNED":
		fallthrough
	case "SMALLINT UNSIGNED":
		fallthrough
	case "MEDIUMINT UNSIGNED":
		fallthrough
	case "INT UNSIGNED":
		fallthrough
	case "BIGINT UNSIGNED":
		return func(field *rdsdataservice.Field) (interface{}, error) {
			return uint64(*field.LongValue), nil
		}
	case "TINYINT":
		fallthrough
	case "SMALLINT":
		fallthrough
	case "MEDIUMINT":
		fallthrough
	case "INT":
		fallthrough
	case "BIGINT":
		return func(field *rdsdataservice.Field) (interface{}, error) {
			return *field.LongValue, nil
		}
	case "DECIMAL":
		return func(field *rdsdataservice.Field) (interface{}, error) {
			return strconv.ParseFloat(aws.StringValue(field.StringValue), 64)
		}
	case "FLOAT":
		fallthrough
	case "DOUBLE":
		return func(field *rdsdataservice.Field) (interface{}, error) {
			return *field.DoubleValue, nil
		}
	case "BIT":
		// Bit values appear to be returned as boolean values
		return func(field *rdsdataservice.Field) (interface{}, error) {
			return nil, errors.New("column types BIT(M) and BOOLEAN are not correctly supported by the Data API")
		}
	case "TINYTEXT":
		fallthrough
	case "TEXT":
		fallthrough
	case "MEDIUMTEXT":
		fallthrough
	case "LONGTEXT":
		fallthrough
	case "CHAR":
		fallthrough
	case "VARCHAR":
		fallthrough
	case "DATE":
		fallthrough
	case "TIME":
		fallthrough
	case "DATETIME":
		fallthrough
	case "TIMESTAMP":
		return func(field *rdsdataservice.Field) (interface{}, error) {
			return *field.StringValue, nil
		}
	case "BINARY":
		fallthrough
	case "VARBINARY":
		fallthrough
	case "TINYBLOB":
		fallthrough
	case "BLOB":
		fallthrough
	case "MEDIUMBLOB":
		fallthrough
	case "LONGBLOB":
		return func(field *rdsdataservice.Field) (interface{}, error) {
			return field.BlobValue, nil
		}
	case "YEAR":
		// RDS sends a full date string. MySQL only returns the year.
		return func(field *rdsdataservice.Field) (interface{}, error) {
			t, err := time.Parse("2006-01-02", aws.StringValue(field.StringValue))
			if err != nil {
				return nil, err
			}
			return strconv.Itoa(t.Year()), nil
		}
	}
	return func(field *rdsdataservice.Field) (interface{}, error) {
		return nil, fmt.Errorf("unknown type %s, please submit a PR", columnType)
	}
}

// IsIsolationLevelSupported for mysql?
func (d *DialectMySQL) IsIsolationLevelSupported(level driver.IsolationLevel) bool {
	// SupportedIsolationLevels for the dialect
	var SupportedIsolationLevels = map[driver.IsolationLevel]bool{
		driver.IsolationLevel(sql.LevelDefault):         true,
		driver.IsolationLevel(sql.LevelRepeatableRead):  true,
		driver.IsolationLevel(sql.LevelReadCommitted):   true,
		driver.IsolationLevel(sql.LevelReadUncommitted): true,
		driver.IsolationLevel(sql.LevelSerializable):    true,
	}
	_, ok := SupportedIsolationLevels[level]
	return ok
}