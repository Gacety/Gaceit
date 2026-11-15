//go:build !darwin

package nativeapp

func startInkSampling() func() { return func() {} }
