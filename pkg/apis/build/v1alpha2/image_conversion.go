package v1alpha2
//
//import (
//	"context"
//	"fmt"
//	"knative.dev/pkg/apis"
//)
//
//func (i *Image) ConvertTo(ctx context.Context, to apis.Convertible) error {
//	switch toI := to.(type) {
//	case *v1alpha1.Image:
//		//toI.TypeMeta = i.TypeMeta
//		//toI.Spec = i.Spec.ImageSpec
//		//toI.Spec.Build.Env = i.Spec.Build.Env
//		//toI.Spec.Build.Resources = i.Spec.Build.Resources
//		//for _, s := range i.Spec.Build.Services {
//		//	toI.Spec.Build.Bindings = append(toI.Spec.Build.Bindings, v1alpha1.Binding{
//		//		Name: s.Name,
//		//		SecretRef: &v1.LocalObjectReference{
//		//			Name: s.Name,
//		//		},
//		//	})
//		//}
//		return fmt.Errorf("converting to v1alpha1 not supported")
//	default:
//		return fmt.Errorf("unsupported type %T", toI)
//	}
//	return nil
//}
//
//func (i *Image) ConvertFrom(ctx context.Context, from apis.Convertible) error {
//	switch fromI := from.(type) {
//	case *v1alpha1.Image:
//		i.TypeMeta = fromI.TypeMeta
//		i.Spec.ImageSpec = fromI.Spec
//		i.Spec.Build.Env = fromI.Spec.Build.Env
//		i.Spec.Build.Resources = fromI.Spec.Build.Resources
//		for _, b := range fromI.Spec.Build.Bindings {
//			i.Spec.Build.Services = append(i.Spec.Build.Services, Service{
//				Name: b.SecretRef.Name,
//				Kind: "Secret",
//			})
//		}
//	default:
//		return fmt.Errorf("unsupported type %T", fromI)
//	}
//	return nil
//}
