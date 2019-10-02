package cnb

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/buildpack/imgutil"
	"github.com/buildpack/lifecycle"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
)

type ImageRebaser struct {
	RemoteImageFactory RemoteImageUtilFactory
}

func (f *ImageRebaser) Rebase(build *v1alpha1.Build, ctx context.Context) (BuiltImage, error) {
	builderImage, err := f.RemoteImageFactory.NewRemote(build.Spec.Builder.Image, registry.SecretRef{
		Namespace:        build.Namespace,
		ImagePullSecrets: build.Spec.Builder.ImagePullSecrets,
		ServiceAccount:   build.Spec.ServiceAccount,
	})
	if err != nil {
		return BuiltImage{}, err
	}

	appImage, err := f.RemoteImageFactory.NewRemote(build.Spec.LastBuild.Image, registry.SecretRef{
		ServiceAccount: build.Spec.ServiceAccount,
		Namespace:      build.Namespace,
	})
	if err != nil {
		return BuiltImage{}, err
	}

	metadataJSON, err := builderImage.Label(BuilderMetadataLabel)
	if err != nil {
		return BuiltImage{}, errors.Wrap(err, "builder image metadata label not present")
	}

	var metadata BuilderImageMetadata
	err = json.Unmarshal([]byte(metadataJSON), &metadata)
	if err != nil {
		return BuiltImage{}, errors.Wrap(err, "unsupported builder metadata structure")
	}

	newBaseImage, err := f.RemoteImageFactory.NewRemote(metadata.Stack.RunImage.Image, registry.SecretRef{
		Namespace:        build.Namespace,
		ImagePullSecrets: build.Spec.Builder.ImagePullSecrets,
		ServiceAccount:   build.Spec.ServiceAccount,
	})
	if err != nil {
		return BuiltImage{}, errors.Wrap(err, "unable to fetch remote run image")
	}

	rebaser := lifecycle.Rebaser{
		Logger: wrappedLogger{logging.FromContext(ctx)},
	}
	err = rebaser.Rebase(appImage, newBaseImage, build.Spec.Tags)
	if err != nil {
		return BuiltImage{}, err
	}

	rebasedImage, err := f.RemoteImageFactory.NewRemote(build.Tag(), registry.SecretRef{
		ServiceAccount: build.Spec.ServiceAccount,
		Namespace:      build.Namespace,
	})
	if err != nil {
		return BuiltImage{}, err
	}

	return readBuiltImage(remoteImageWrapper{rebasedImage})
}

type remoteImageWrapper struct {
	remoteImgUtilImage imgutil.Image
}

func (r remoteImageWrapper) CreatedAt() (time.Time, error) {
	return r.remoteImgUtilImage.CreatedAt()
}

func (r remoteImageWrapper) Identifier() (string, error) {
	i, err := r.remoteImgUtilImage.Identifier()
	if err != nil {
		return "", err
	}
	return i.String(), nil
}

func (r remoteImageWrapper) Label(labelName string) (string, error) {
	return r.remoteImgUtilImage.Label(labelName)
}

func (r remoteImageWrapper) Env(key string) (string, error) {
	return r.remoteImgUtilImage.Env(key)
}

type wrappedLogger struct {
	logger *zap.SugaredLogger
}

func (w wrappedLogger) Debug(msg string) {
	w.logger.Debug(msg)
}

func (w wrappedLogger) Debugf(fmt string, v ...interface{}) {
	w.logger.Debugf(fmt, v)
}

func (w wrappedLogger) Info(msg string) {
	w.logger.Info(msg)
}

func (w wrappedLogger) Infof(fmt string, v ...interface{}) {
	w.logger.Infof(fmt, v)
}

func (w wrappedLogger) Warn(msg string) {
	w.logger.Warn(msg)
}

func (w wrappedLogger) Warnf(fmt string, v ...interface{}) {
	w.logger.Warnf(fmt, v)
}

func (w wrappedLogger) Error(msg string) {
	w.logger.Error(msg)
}

func (w wrappedLogger) Errorf(fmt string, v ...interface{}) {
	w.logger.Errorf(fmt, v)
}

func (w wrappedLogger) Writer() io.Writer {
	panic("implement me")
}

func (w wrappedLogger) WantLevel(level string) {
	panic("implement me")
}
