package s3

import (
	"io/ioutil"
	"log"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal/kpack/pkg/util"
)

type Fetcher struct {
	Logger *log.Logger
}

func (f *Fetcher) Fetch(dir string, s3URL string, s3AccessKey string, s3SecretKey string, s3Bucket string, s3File string, s3ForcePathStyle bool, s3Region string) error {
	blob, err := url.Parse(s3URL)
	if err != nil {
		return err
	}

	option := session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}

	region := s3Region
	if region == "" {
		region = "s3RegionStub" // Needed by amazon-sdk
	}

	option.Config = aws.Config{
		Endpoint:         aws.String(s3URL),
		S3ForcePathStyle: aws.Bool(s3ForcePathStyle),
		Credentials:      credentials.NewStaticCredentials(s3AccessKey, s3SecretKey, ""),
		Region:           aws.String(region),
	}

	sess, err := session.NewSessionWithOptions(option)
	downloader := s3manager.NewDownloader(sess)

	file, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(file.Name())

	_, err = downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(s3File),
	})

	if err != nil {
		return err
	}

	err = util.ExtractFile(file, dir)
	if err != nil {
		return err
	}

	f.Logger.Printf("Successfully downloaded %s/%s/%s in path %q", blob.Host+blob.Path, s3Bucket, s3File, dir)
	return nil
}
