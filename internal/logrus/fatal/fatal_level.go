package fatal

import "github.com/sirupsen/logrus"

// package can be imported to set the log level to fatal

func init() {
	logrus.SetLevel(logrus.FatalLevel)
}
