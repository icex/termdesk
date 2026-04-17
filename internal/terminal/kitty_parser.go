package terminal

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// ParseKittyCommand parses a Kitty graphics command payload.
// The input is the data between ESC_G and the string terminator (ESC\ or BEL),
// i.e. <control-params>;<base64-payload>.
func ParseKittyCommand(data []byte) (*KittyCommand, error) {
	if len(data) == 0 {
		return nil, nil
	}

	cmd := &KittyCommand{
		Action: KittyActionTransmit,
		Medium: KittyMediumDirect,
		Format: KittyFormatRGBA,
	}

	controlPart, dataPart, _ := bytes.Cut(data, []byte{';'})

	if len(controlPart) > 0 {
		parseKittyControlParams(string(controlPart), cmd)
	}

	if len(dataPart) > 0 {
		switch cmd.Medium {
		case KittyMediumFile, KittyMediumTempFile, KittyMediumSharedMemory:
			decoded, err := base64.StdEncoding.DecodeString(string(dataPart))
			if err == nil {
				cmd.FilePath = string(decoded)
			} else {
				// Retry without padding — some implementations omit '=' padding.
				decoded, err2 := base64.RawStdEncoding.DecodeString(string(dataPart))
				if err2 == nil {
					cmd.FilePath = string(decoded)
				} else {
					kittyPassDbg("ParseKittyCommand: base64 decode failed for file path: std=%v raw=%v dataPartLen=%d dataPart=%q",
						err, err2, len(dataPart), dataPart)
				}
			}
		default:
			decoded, err := base64.StdEncoding.DecodeString(string(dataPart))
			if err == nil {
				cmd.Data = decoded
			} else {
				cmd.Data = dataPart
			}
		}
	} else if cmd.Medium == KittyMediumFile || cmd.Medium == KittyMediumTempFile {
		kittyPassDbg("ParseKittyCommand: file medium %c but no data part (rawLen=%d)", byte(cmd.Medium), len(data))
	}

	return cmd, nil
}

func parseKittyControlParams(control string, cmd *KittyCommand) {
	for _, pair := range strings.Split(control, ",") {
		if pair == "" {
			continue
		}
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		switch key {
		case "a":
			if len(value) > 0 {
				cmd.Action = KittyGraphicsAction(value[0])
			}
		case "q":
			cmd.Quiet, _ = strconv.Atoi(value)
		case "i":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				cmd.ImageID = uint32(v)
			}
		case "I":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				cmd.ImageNumber = uint32(v)
			}
		case "p":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				cmd.PlacementID = uint32(v)
			}
		case "f":
			if v, err := strconv.Atoi(value); err == nil {
				cmd.Format = KittyGraphicsFormat(v)
			}
		case "t":
			if len(value) > 0 {
				cmd.Medium = KittyGraphicsMedium(value[0])
			}
		case "o":
			if len(value) > 0 && value[0] == 'z' {
				cmd.Compression = KittyCompressionZlib
			}
		case "s":
			cmd.Width, _ = strconv.Atoi(value)
		case "v":
			cmd.Height, _ = strconv.Atoi(value)
		case "S":
			cmd.Size, _ = strconv.Atoi(value)
		case "O":
			cmd.Offset, _ = strconv.Atoi(value)
		case "m":
			cmd.More = value == "1"
		case "d":
			if len(value) > 0 {
				cmd.Delete = KittyDeleteTarget(value[0])
			}
		case "x":
			cmd.SourceX, _ = strconv.Atoi(value)
		case "y":
			cmd.SourceY, _ = strconv.Atoi(value)
		case "w":
			cmd.SourceWidth, _ = strconv.Atoi(value)
		case "h":
			cmd.SourceHeight, _ = strconv.Atoi(value)
		case "X":
			cmd.XOffset, _ = strconv.Atoi(value)
		case "Y":
			cmd.YOffset, _ = strconv.Atoi(value)
		case "c":
			cmd.Columns, _ = strconv.Atoi(value)
		case "r":
			cmd.Rows, _ = strconv.Atoi(value)
		case "z":
			if v, err := strconv.ParseInt(value, 10, 32); err == nil {
				cmd.ZIndex = int32(v)
			}
		case "C":
			cmd.CursorMove, _ = strconv.Atoi(value)
		case "U":
			cmd.Virtual = value == "1"
		}
	}
}

// BuildKittyResponse builds a Kitty graphics protocol response.
func BuildKittyResponse(ok bool, imageID uint32, message string) []byte {
	var buf bytes.Buffer
	buf.WriteString("\x1b_G")
	if imageID > 0 {
		fmt.Fprintf(&buf, "i=%d;", imageID)
	}
	if ok {
		buf.WriteString("OK")
	} else if message != "" {
		buf.WriteString(message)
	} else {
		buf.WriteString("ENOENT:file not found")
	}
	buf.WriteString("\x1b\\")
	return buf.Bytes()
}

// isKittyResponse checks if data looks like a kitty graphics protocol response
// (OK or POSIX error like ENOENT) rather than real image data.
func isKittyResponse(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if string(data) == "OK" {
		return true
	}
	return len(data) >= 2 && data[0] == 'E' && data[1] >= 'A' && data[1] <= 'Z'
}
