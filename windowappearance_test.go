package main

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestMacWindowOptions(t *testing.T) {
	options := macWindowOptions()
	if options.Backdrop != application.MacBackdropTranslucent {
		t.Fatal("macOS window must use the frosted translucent backdrop")
	}
	if options.TitleBar != application.MacTitleBarHiddenInset || options.InvisibleTitleBarHeight != 50 {
		t.Error("glass must preserve the existing title bar layout")
	}
}

func TestWindowBackgroundColour(t *testing.T) {
	for _, platform := range []string{"darwin", "windows", "linux"} {
		t.Run(platform, func(t *testing.T) {
			colour := windowBackgroundColour(platform)
			want := application.NewRGB(0, 0, 0)
			if platform == "darwin" {
				want = application.NewRGBA(0, 0, 0, 0)
			}
			if colour != want {
				t.Errorf("background = %+v, want %+v", colour, want)
			}
		})
	}
}
