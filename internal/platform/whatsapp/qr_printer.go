package whatsapp

import (
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// PrintQRASCII renders the QR code in a compact ASCII form for terminal scanning.
func PrintQRASCII(code string) {
	qr, err := qrcode.New(code, qrcode.Medium)
	if err != nil {
		fmt.Println("[QR] erro ao gerar QR:", err)
		return
	}
	qr.DisableBorder = true
	bmp := qr.Bitmap()
	if len(bmp)%2 == 1 {
		width := 0
		if len(bmp) > 0 {
			width = len(bmp[0])
		}
		padding := make([]bool, width)
		bmp = append(bmp, padding)
	}

	fmt.Println("\n[QR] Escaneie (compacto):")
	for y := 0; y < len(bmp); y += 2 {
		top := bmp[y]
		bottom := bmp[y+1]
		var line strings.Builder
		for x := 0; x < len(top); x++ {
			t := top[x]
			b := bottom[x]
			switch {
			case t && b:
				line.WriteRune('█')
			case t && !b:
				line.WriteRune('▀')
			case !t && b:
				line.WriteRune('▄')
			default:
				line.WriteRune(' ')
			}
		}
		fmt.Println(line.String())
	}
	fmt.Println()
}
