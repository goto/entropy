package firehose

import (
	"reflect"

	"github.com/imdario/mergo"
)

type kedaConfigMergeTransformers struct{}

func (kedaConfigMergeTransformers) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(map[string]Trigger{}) {
		return mapTriggerTransformer
	}

	if typ == reflect.TypeOf(&HorizontalPodAutoscaler{}) {
		return hpaTransformer
	}

	if typ == reflect.TypeOf(&Fallback{}) {
		return fallbackTransformer
	}
	return nil
}

func mapTriggerTransformer(dst, src reflect.Value) error {
	for _, key := range src.MapKeys() {
		srcVal := src.MapIndex(key).Interface().(Trigger)
		if dstVal := dst.MapIndex(key); dstVal.IsValid() {
			merged := dstVal.Interface().(Trigger)

			if merged.Metadata == nil {
				merged.Metadata = map[string]string{}
			}
			for k, v := range srcVal.Metadata {
				merged.Metadata[k] = v
			}

			if srcVal.AuthenticationRef.Name != "" {
				merged.AuthenticationRef = srcVal.AuthenticationRef
			}

			if srcVal.Type != "" {
				merged.Type = srcVal.Type
			}

			dst.SetMapIndex(key, reflect.ValueOf(merged))
		} else {
			dst.SetMapIndex(key, reflect.ValueOf(srcVal))
		}
	}
	return nil
}

func hpaTransformer(dst, src reflect.Value) error {
	if src.IsNil() {
		return nil
	}
	if dst.IsNil() {
		dst.Set(src)
		return nil
	}

	dstHPA := dst.Interface().(*HorizontalPodAutoscaler)
	srcHPA := src.Interface().(*HorizontalPodAutoscaler)

	if len(srcHPA.ScaleUp.Policies) > 0 ||
		srcHPA.ScaleUp.StabilizationWindowSeconds != nil ||
		srcHPA.ScaleUp.Tolerance != nil {
		err := mergo.Merge(&dstHPA.ScaleUp, srcHPA.ScaleUp, mergo.WithOverride)
		if err != nil {
			return err
		}
	}

	if len(srcHPA.ScaleDown.Policies) > 0 ||
		srcHPA.ScaleDown.StabilizationWindowSeconds != nil ||
		srcHPA.ScaleDown.Tolerance != nil {
		err := mergo.Merge(&dstHPA.ScaleDown, srcHPA.ScaleDown, mergo.WithOverride)
		if err != nil {
			return err
		}
	}

	return nil
}

func fallbackTransformer(dst, src reflect.Value) error {
	if src.IsNil() {
		return nil
	}
	if dst.IsNil() {
		dst.Set(src)
		return nil
	}

	dstFallback := dst.Interface().(*Fallback)
	srcFallback := src.Interface().(*Fallback)

	if srcFallback.Behavior != "" {
		dstFallback.Behavior = srcFallback.Behavior
	}

	if srcFallback.Replicas != 0 {
		dstFallback.Replicas = srcFallback.Replicas
	}

	if srcFallback.FailureThreshold != 0 {
		dstFallback.FailureThreshold = srcFallback.FailureThreshold
	}

	return nil
}
