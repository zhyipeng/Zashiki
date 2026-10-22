package appupdate

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

const Repository = "zhyipeng/Zashiki"

// 更新包与首次安装包分开匹配，避免把 NSIS 安装程序当成应用本体替换。
func matchAsset(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	name := ""
	switch req.Platform {
	case "darwin":
		if req.Arch == "arm64" || req.Arch == "amd64" {
			name = "Zashiki-darwin-universal-update.zip"
		}
	case "windows":
		if req.Arch == "amd64" {
			name = "Zashiki-windows-amd64-update.zip"
		}
	case "linux":
		if req.Arch == "amd64" {
			name = "Zashiki-linux-amd64-update.tar.gz"
		}
	}
	for i, asset := range assets {
		if name != "" && asset.Name == name {
			return i
		}
	}
	return -1
}

type verifiedProvider struct{ updater.Provider }

func (p verifiedProvider) Check(ctx context.Context, req updater.CheckRequest) (*updater.Release, error) {
	release, err := p.Provider.Check(ctx, req)
	if err != nil || release == nil {
		return release, err
	}
	if release.Verification == nil || release.Verification.DigestAlgo != "sha256" || len(release.Verification.Digest) != sha256.Size {
		return nil, errors.New("更新包缺少有效的 SHA-256 校验，请从发布页下载安装")
	}
	return release, nil
}
