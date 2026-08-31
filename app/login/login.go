package login

import (
	"context"

	"github.com/go-faster/errors"
)

//go:generate go-enum --values --names --flag --nocase

// Type
// ENUM(desktop, code, qr)
type Type int

type Options struct {
	Type     Type
	Desktop  string
	Passcode string
	// WebUIQR makes the QR login print the login URL as a machine-readable
	// WEBUI_QR: line instead of a terminal QR code, for the WebUI server.
	WebUIQR bool
}

func Run(ctx context.Context, opts Options) error {
	switch opts.Type {
	case TypeDesktop:
		return Desktop(ctx, opts)
	case TypeCode:
		return Code(ctx)
	case TypeQr:
		return QR(ctx, opts.WebUIQR)
	default:
		return errors.Errorf("unsupported login type: %s", opts.Type)
	}
}
