package ui

import (
	"embed"
	"fmt"
	"math/rand"
)

//go:embed banners/*.txt
var bannerDir embed.FS

func randomBanner() string {
	bannerFiles, _ := bannerDir.ReadDir("banners")
	file, _ := bannerDir.ReadFile("banners/" + bannerFiles[rand.Intn(len(bannerFiles))].Name())

	return fmt.Sprintf("Welcome to...\n\n%s\n\n", file)
}
