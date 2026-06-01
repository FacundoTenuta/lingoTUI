package config

import "github.com/FacundoTenuta/lingoTUI/internal/app"

const FileName = "config.json"

func Default() app.Config { return app.DefaultConfig() }
