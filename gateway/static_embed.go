package gateway

import "embed"

//go:embed static/*
var embeddedStaticFS embed.FS
