package rds_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rdsdataservice"
	"github.com/krotscheck/go-rds-driver"
	. "github.com/smartystreets/goconvey/convey"
	"log"
	"os"
	"testing"
)

// TestConfig to use when making integration test calls
var TestConfig *rds.Config

func init() {
	// Testing requires a few environment parameters
	resourceARN := os.Getenv("RDS_TEST_RESOURCE_ARN")
	if resourceARN == "" {
		log.Fatal("Missing test environment parameter: RDS_TEST_RESOURCE_ARN")
	}
	secretARN := os.Getenv("RDS_TEST_SECRET_ARN")
	if secretARN == "" {
		log.Fatal("Missing test environment parameter: RDS_TEST_SECRET_ARN")
	}
	database := os.Getenv("RDS_TEST_DATABASE")
	if database == "" {
		log.Fatal("Missing test environment parameter: RDS_TEST_DATABASE")
	}
	region := os.Getenv("AWS_REGION")
	if region == "" {
		log.Fatal("Missing test environment parameter: AWS_REGION")
	}

	TestConfig = rds.NewConfig(resourceARN, secretARN, database, region)

	// Make sure the database exists...
	awsConfig := aws.NewConfig().WithRegion(TestConfig.AWSRegion)
	awsSession, err := session.NewSession(awsConfig)
	if err != nil {
		log.Fatal(err)
	}
	rdsAPI := rdsdataservice.New(awsSession)

	// Wakeup the cluster
	_, err = rds.Wakeup(rdsAPI, resourceARN, secretARN, database)
	if err != nil {
		log.Fatal(err)
	}

	_, err = rdsAPI.ExecuteStatement(&rdsdataservice.ExecuteStatementInput{
		ResourceArn: aws.String(TestConfig.ResourceArn),
		SecretArn:   aws.String(TestConfig.SecretArn),
		Sql:         aws.String(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", TestConfig.Database)),
	})
	if err != nil {
		log.Fatal(err)
	}
}

// ExpectWakeup can be used whenever we're mocking out a new connection
func ExpectWakeup(mockRDS *MockRDSDataServiceAPI, conf *rds.Config) {
	mockRDS.EXPECT().
		ExecuteStatement(ExpectedStatement(conf, "/* wakeup */ SELECT VERSION()", nil)).
		AnyTimes().
		Return(&rdsdataservice.ExecuteStatementOutput{
			Records: [][]*rdsdataservice.Field{
				{
					&rdsdataservice.Field{
						StringValue: aws.String("5.7.0"),
					},
				},
			},
		}, nil)
}

// ExpectTransaction to be started
func ExpectTransaction(ctx context.Context, mockRDS *MockRDSDataServiceAPI, conf *rds.Config, transactionID string, readonly bool, isolation sql.IsolationLevel) {
	mockRDS.EXPECT().
		BeginTransactionWithContext(ctx, &rdsdataservice.BeginTransactionInput{
			Database:    aws.String(conf.Database),
			ResourceArn: aws.String(conf.ResourceArn),
			SecretArn:   aws.String(conf.SecretArn),
		}).
		Times(1).
		Return(&rdsdataservice.BeginTransactionOutput{TransactionId: aws.String(transactionID)}, nil)

	mockRDS.EXPECT().
		ExecuteStatementWithContext(ctx, &rdsdataservice.ExecuteStatementInput{
			Database:    aws.String(conf.Database),
			ResourceArn: aws.String(conf.ResourceArn),
			SecretArn:   aws.String(conf.SecretArn),
			Sql:         aws.String("SET TRANSACTION ISOLATION LEVEL :isolation, :readonly"),
			Parameters: []*rdsdataservice.SqlParameter{
				{
					Name:  aws.String("isolation"),
					Value: &rdsdataservice.Field{StringValue: aws.String(isolation.String())},
				},
				{
					Name:  aws.String("readonly"),
					Value: &rdsdataservice.Field{StringValue: aws.String("READ WRITE")},
				},
			},
		}).
		Times(1).
		Return(&rdsdataservice.BeginTransactionOutput{TransactionId: aws.String(transactionID)}, nil)
}

// ExpectQuery in a test
func ExpectedStatement(conf *rds.Config, query string, args []driver.NamedValue) *rdsdataservice.ExecuteStatementInput {
	params, err := rds.ConvertNamedValues(args)
	So(err, ShouldBeNil)
	return &rdsdataservice.ExecuteStatementInput{
		Database:    aws.String(conf.Database),
		ResourceArn: aws.String(conf.ResourceArn),
		SecretArn:   aws.String(conf.SecretArn),
		Sql:         aws.String(query),
		Parameters:  params,
	}
}

func Test_ConvertQuery(t *testing.T) {

	Convey("ConvertQuery", t, func() {
		Convey("All Ordinal", func() {
			inputQuery := "SELECT ? FROM ? WHERE id = ?"
			inputArgs := []driver.NamedValue{
				{Name: "", Ordinal: 1, Value: "name"},
				{Name: "", Ordinal: 2, Value: "my_table"},
				{Name: "", Ordinal: 3, Value: "unique_id"},
			}
			expected := &rdsdataservice.ExecuteStatementInput{
				Parameters: []*rdsdataservice.SqlParameter{
					{
						Name: aws.String("1"),
						Value: &rdsdataservice.Field{
							StringValue: aws.String("name"),
						},
					},
					{
						Name: aws.String("2"),
						Value: &rdsdataservice.Field{
							StringValue: aws.String("my_table"),
						},
					},
					{
						Name: aws.String("3"),
						Value: &rdsdataservice.Field{
							StringValue: aws.String("unique_id"),
						},
					},
				},
				Sql: aws.String("SELECT :1 FROM :2 WHERE id = :3"),
			}

			output, err := rds.MigrateQuery(inputQuery, inputArgs)
			So(err, ShouldBeNil)
			So(output, ShouldResemble, expected)
		})
		Convey("All Named", func() {
			inputQuery := "SELECT :field FROM :table WHERE id = :id"
			inputArgs := []driver.NamedValue{
				{Name: "field", Value: "name"},
				{Name: "table", Value: "my_table"},
				{Name: "id", Value: "unique_id"},
			}
			expected := &rdsdataservice.ExecuteStatementInput{
				Parameters: []*rdsdataservice.SqlParameter{
					{
						Name: aws.String("field"),
						Value: &rdsdataservice.Field{
							StringValue: aws.String("name"),
						},
					},
					{
						Name: aws.String("table"),
						Value: &rdsdataservice.Field{
							StringValue: aws.String("my_table"),
						},
					},
					{
						Name: aws.String("id"),
						Value: &rdsdataservice.Field{
							StringValue: aws.String("unique_id"),
						},
					},
				},
				Sql: aws.String("SELECT :field FROM :table WHERE id = :id"),
			}

			output, err := rds.MigrateQuery(inputQuery, inputArgs)
			So(err, ShouldBeNil)
			So(output, ShouldResemble, expected)
		})
		Convey("Mixed", func() {
			inputQuery := "SELECT ? FROM :table WHERE id = :id"
			inputArgs := []driver.NamedValue{
				{Ordinal: 1, Value: "name"},
				{Name: "table", Value: "my_table"},
				{Name: "id", Value: "unique_id"},
			}

			_, err := rds.MigrateQuery(inputQuery, inputArgs)
			So(err, ShouldNotBeNil)
		})
	})
}
