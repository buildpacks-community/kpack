package v1alpha2

//import (
//	"context"
//	"fmt"
//
//	"knative.dev/pkg/apis"
//)
//
//func (b *Build) ConvertTo(ctx context.Context, to apis.Convertible) error {
//	switch toI := to.(type) {
//	case *Build:
//		//toI.TypeMeta = b.TypeMeta
//		//toI.Spec = b.Spec.BuildSpec
//		//toI.Spec.Build.Env = b.Spec.Build.Env
//		//toI.Spec.Build.Resources = b.Spec.Build.Resources
//		//for _, s := range b.Spec.Build.Services {
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
//func (b *Build) ConvertFrom(ctx context.Context, from apis.Convertible) error {
//	switch fromB := from.(type) {
//	case *Build:
//		b.TypeMeta = fromB.TypeMeta
//		b.Spec.BuildSpec = fromB.Spec
//		b.Spec.Env = fromB.Spec.Env
//		b.Spec.Resources = fromB.Spec.Resources
//		for _, bnd := range fromB.Spec.Bindings {
//			b.Spec.Services = append(b.Spec.Services, Service{
//				Name: bnd.SecretRef.Name,
//				Kind: "Secret",
//			})
//		}
//	default:
//		return fmt.Errorf("unsupported type %T", fromB)
//	}
//	return nil
//}
