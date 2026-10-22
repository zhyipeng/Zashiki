package main

import "github.com/wailsapp/wails/v3/pkg/application"

func macWindowOptions() application.MacWindow {
	return application.MacWindow{
		InvisibleTitleBarHeight: 50,
		Backdrop:                application.MacBackdropTranslucent,
		TitleBar:                application.MacTitleBarHiddenInset,
	}
}

func windowBackgroundColour(platform string) application.RGBA {
	if platform == "darwin" {
		return application.NewRGBA(0, 0, 0, 0)
	}
	return application.NewRGB(0, 0, 0)
}
