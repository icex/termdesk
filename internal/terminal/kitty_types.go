package terminal

// Kitty graphics protocol types and constants.
// See: https://sw.kovidgoyal.net/kitty/graphics-protocol/

// KittyGraphicsAction defines the action type (a= parameter).
type KittyGraphicsAction byte

const (
	KittyActionTransmit      KittyGraphicsAction = 't' // transmit data
	KittyActionTransmitPlace KittyGraphicsAction = 'T' // transmit + place
	KittyActionQuery         KittyGraphicsAction = 'q' // query support
	KittyActionPlace         KittyGraphicsAction = 'p' // place previously transmitted
	KittyActionDelete        KittyGraphicsAction = 'd' // delete image(s)
	KittyActionFrame         KittyGraphicsAction = 'f' // animation frame
	KittyActionAnimate       KittyGraphicsAction = 'a' // animation control
	KittyActionCompose       KittyGraphicsAction = 'c' // compose frames
)

// KittyGraphicsFormat defines image data format (f= parameter).
type KittyGraphicsFormat int

const (
	KittyFormatRGBA KittyGraphicsFormat = 32
	KittyFormatRGB  KittyGraphicsFormat = 24
	KittyFormatPNG  KittyGraphicsFormat = 100
)

// KittyGraphicsMedium defines data transmission medium (t= parameter).
type KittyGraphicsMedium byte

const (
	KittyMediumDirect       KittyGraphicsMedium = 'd' // direct (base64 in payload)
	KittyMediumFile         KittyGraphicsMedium = 'f' // regular file
	KittyMediumTempFile     KittyGraphicsMedium = 't' // temp file (auto-deleted)
	KittyMediumSharedMemory KittyGraphicsMedium = 's' // POSIX shared memory
)

// KittyGraphicsCompression defines compression (o= parameter).
type KittyGraphicsCompression byte

const (
	KittyCompressionNone KittyGraphicsCompression = 0
	KittyCompressionZlib KittyGraphicsCompression = 'z'
)

// KittyDeleteTarget defines what to delete (d= parameter).
type KittyDeleteTarget byte

const (
	KittyDeleteAll              KittyDeleteTarget = 'a' // all images
	KittyDeleteByID             KittyDeleteTarget = 'i' // by image ID
	KittyDeleteByIDAndPlacement KittyDeleteTarget = 'I' // by ID + placement ID
)

// KittyCommand represents a parsed Kitty graphics command.
type KittyCommand struct {
	Action       KittyGraphicsAction
	Quiet        int // suppress responses (q=)
	ImageID      uint32
	ImageNumber  uint32
	PlacementID  uint32
	Format       KittyGraphicsFormat
	Medium       KittyGraphicsMedium
	Compression  KittyGraphicsCompression
	Width        int // pixel width (s=)
	Height       int // pixel height (v=)
	Size         int // data size (S=)
	Offset       int // data offset (O=)
	More         bool
	Delete       KittyDeleteTarget
	SourceX      int // source rect x (x=)
	SourceY      int // source rect y (y=)
	SourceWidth  int // source rect width (w=)
	SourceHeight int // source rect height (h=)
	XOffset      int // cell offset x (X=)
	YOffset      int // cell offset y (Y=)
	Columns      int // display columns (c=)
	Rows         int // display rows (r=)
	ZIndex       int32
	CursorMove   int  // cursor movement (C=)
	Virtual      bool // virtual placement (U=)
	Data         []byte
	FilePath     string
}
