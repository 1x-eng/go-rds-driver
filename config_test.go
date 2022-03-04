package rds_test

import (
	"testing"

	"github.com/1x-eng/go-rds-driver"
	. "github.com/smartystreets/goconvey/convey"
)

func Test_Config(t *testing.T) {
	Convey("Config", t, func() {
		conf := rds.NewConfig("resourceARN", "secretARN", "database", "region")
		dsn, err := conf.ToDSN()
		So(err, ShouldBeNil)
		conf1, err := rds.NewConfigFromDSN(dsn)
		So(err, ShouldBeNil)
		So(conf, ShouldResemble, conf1)
	})
}
