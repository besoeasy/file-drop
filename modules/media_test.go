package modules

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAnonymizeJPEGStripsExifAndGPS(t *testing.T) {
	src := filepath.Join(t.TempDir(), "shot.jpg")
	raw := jpegWithExif(t, testBarImage(), 6)
	if !bytes.Contains(raw, []byte("Exif\x00\x00")) {
		t.Fatal("fixture is missing EXIF")
	}
	if !bytes.Contains(raw, []byte("GPS")) && !bytes.Contains(raw, []byte{0x4E, 0x00}) {
		t.Fatal("fixture is missing GPS latitude ref")
	}
	if err := os.WriteFile(src, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := AnonymizeImage(src, "IMG_GPS.JPG")
	if err != nil {
		t.Fatal(err)
	}
	defer RemovePath(out.Path)

	got, err := os.ReadFile(out.Path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("Exif\x00\x00")) {
		t.Fatal("anonymized jpeg still contains EXIF")
	}
	if bytes.Contains(got, []byte("GPS")) {
		t.Fatal("anonymized jpeg still contains GPS")
	}
	if out.Mime != "image/jpeg" {
		t.Fatalf("mime = %q", out.Mime)
	}
	if out.Orientation != 6 {
		t.Fatalf("orientation = %d, want 6", out.Orientation)
	}

	img, err := jpeg.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != 16 || b.Dy() != 32 {
		t.Fatalf("oriented size = %dx%d, want 16x32", b.Dx(), b.Dy())
	}
	top := img.At(b.Min.X+8, b.Min.Y+4)
	bot := img.At(b.Min.X+8, b.Min.Y+28)
	if !isMostlyRed(top) {
		t.Fatalf("top after orientation 6 is not red: %#v", top)
	}
	if !isMostlyBlue(bot) {
		t.Fatalf("bottom after orientation 6 is not blue: %#v", bot)
	}
}

func TestAnonymizePNGStripsTextChunks(t *testing.T) {
	src := filepath.Join(t.TempDir(), "photo.png")
	raw := pngWithText(t, testBarImage(), "Author", "Secret Photographer")
	if !bytes.Contains(raw, []byte("Secret Photographer")) {
		t.Fatal("fixture is missing tEXt payload")
	}
	if err := os.WriteFile(src, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := AnonymizeImage(src, "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	defer RemovePath(out.Path)

	got, err := os.ReadFile(out.Path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("Secret Photographer")) || bytes.Contains(got, []byte("Author")) {
		t.Fatal("anonymized png still contains text metadata")
	}
	if _, err := png.Decode(bytes.NewReader(got)); err != nil {
		t.Fatal(err)
	}
	if out.Mime != "image/png" {
		t.Fatalf("mime = %q", out.Mime)
	}
}

func TestAnonymizeRejectsNonImage(t *testing.T) {
	src := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(src, []byte("%PDF-1.4 not an image at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := AnonymizeImage(src, "note.txt")
	if !errors.Is(err, ErrUnsupportedMedia) {
		t.Fatalf("expected unsupported media, got %v", err)
	}
}

func TestMediaHandlerRejectsNonImage(t *testing.T) {
	h := NewHandler(nil, nil, NewMetrics(), nil)
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "note.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("hello originless")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/media", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.Media(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "Unsupported media type" {
		t.Fatalf("error = %#v", resp["error"])
	}
}

func testBarImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 32, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 32; x++ {
			if x < 16 {
				img.Set(x, y, color.RGBA{R: 255, A: 255})
			} else {
				img.Set(x, y, color.RGBA{B: 255, A: 255})
			}
		}
	}
	return img
}

func isMostlyRed(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r > 0xA000 && g < 0x6000 && b < 0x6000
}

func isMostlyBlue(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return b > 0xA000 && r < 0x6000 && g < 0x6000
}

func jpegWithExif(t *testing.T, img image.Image, orientation int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	jpegBytes := buf.Bytes()
	exif := buildExifAPP1(orientation)
	app1 := []byte{0xFF, 0xE1, byte((len(exif) + 2) >> 8), byte(len(exif) + 2)}
	app1 = append(app1, exif...)
	out := make([]byte, 0, 2+len(app1)+len(jpegBytes)-2)
	out = append(out, jpegBytes[:2]...)
	out = append(out, app1...)
	out = append(out, jpegBytes[2:]...)
	return out
}

func buildExifAPP1(orientation int) []byte {
	// TIFF little-endian with Orientation + GPSLatitudeRef='N'
	tiff := make([]byte, 0, 64)
	tiff = append(tiff, 'I', 'I', 0x2A, 0x00)
	tiff = append(tiff, 0x08, 0x00, 0x00, 0x00) // IFD0 at 8
	tiff = append(tiff, 0x02, 0x00)             // 2 entries
	// Orientation 0x0112 SHORT count 1
	tiff = append(tiff, 0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, byte(orientation), 0x00, 0x00, 0x00)
	// GPS IFD pointer 0x8825 at offset 38
	tiff = append(tiff, 0x25, 0x88, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x26, 0x00, 0x00, 0x00)
	tiff = append(tiff, 0x00, 0x00, 0x00, 0x00) // next IFD
	// GPS IFD: GPSLatitudeRef ASCII "N"
	tiff = append(tiff, 0x01, 0x00)
	tiff = append(tiff, 0x01, 0x00, 0x02, 0x00, 0x02, 0x00, 0x00, 0x00, 'N', 0x00, 0x00, 0x00)
	tiff = append(tiff, 0x00, 0x00, 0x00, 0x00)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	return payload
}

func pngWithText(t *testing.T, img image.Image, key, value string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	sig := 8
	// Insert tEXt after IHDR (first chunk).
	if len(data) < sig+12 {
		t.Fatal("png too small")
	}
	ihdrLen := int(binary.BigEndian.Uint32(data[sig : sig+4]))
	afterIHDR := sig + 12 + ihdrLen
	textData := append(append([]byte(key), 0), []byte(value)...)
	chunk := pngChunk("tEXt", textData)
	out := append([]byte{}, data[:afterIHDR]...)
	out = append(out, chunk...)
	out = append(out, data[afterIHDR:]...)
	return out
}

func pngChunk(typ string, payload []byte) []byte {
	chunk := make([]byte, 0, 12+len(payload))
	lenbuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenbuf, uint32(len(payload)))
	chunk = append(chunk, lenbuf...)
	chunk = append(chunk, []byte(typ)...)
	chunk = append(chunk, payload...)
	crc := crc32.NewIEEE()
	crc.Write([]byte(typ))
	crc.Write(payload)
	crcbuf := make([]byte, 4)
	binary.BigEndian.PutUint32(crcbuf, crc.Sum32())
	chunk = append(chunk, crcbuf...)
	return chunk
}
