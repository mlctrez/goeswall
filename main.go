package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	// GOES-East (GOES-19) CONUS GeoColor imagery from NOAA CDN
	baseURL = "https://cdn.star.nesdis.noaa.gov/GOES19/ABI/CONUS/GEOCOLOR/"

	// Timestamp position offset from the top-left corner in pixels
	timestampOffsetX = 100
	timestampOffsetY = 40
)

// Available image sizes from the NOAA CDN
var validSizes = []string{
	"latest",     // default latest.jpg (1250x750)
	"625x375",    // small
	"1250x750",   // medium
	"2500x1500",  // large
	"5000x3000",  // full resolution
	"10000x6000", // ultra high resolution
}

func main() {
	var (
		outputDir string
		size      string
		setWP     bool
		wpMethod  string
		verbose   bool
	)

	flag.StringVar(&outputDir, "output", defaultOutputDir(), "Directory to save the downloaded image")
	flag.StringVar(&size, "size", "5000x3000", "Image size: latest, 625x375, 1250x750, 2500x1500, 5000x3000, 10000x6000")
	flag.BoolVar(&setWP, "set-wallpaper", true, "Set the downloaded image as desktop wallpaper")
	flag.StringVar(&wpMethod, "method", "auto", "Wallpaper method: auto, gnome, kde, xfce, sway, feh, nitrogen")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flag.Parse()

	if !isValidSize(size) {
		fmt.Fprintf(os.Stderr, "Invalid size %q. Valid sizes: %s\n", size, strings.Join(validSizes, ", "))
		os.Exit(1)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Build the image URL
	imageURL := buildImageURL(size)
	if verbose {
		fmt.Printf("Image URL: %s\n", imageURL)
	}

	// Download the image
	rawPath := filepath.Join(outputDir, "goes_conus_geocolor_raw.jpg")
	if err := downloadImage(imageURL, rawPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to download image: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("Image saved to: %s\n", rawPath)
	}

	// Crop upper-left 2/3 and convert to PNG for wallpaper use
	// (GNOME's gdk-pixbuf has issues rendering large JPEGs, and we only
	// want the continental US portion without the Atlantic/Mexico)
	wallpaperPath := filepath.Join(outputDir, "goes_conus_geocolor.png")
	if err := cropAndConvert(rawPath, wallpaperPath, verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to process image: %v\n", err)
		os.Exit(1)
	}

	// Set as wallpaper
	if setWP {
		if err := setWallpaper(wallpaperPath, wpMethod, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set wallpaper: %v\n", err)
			os.Exit(1)
		}
		if verbose {
			fmt.Println("Wallpaper updated successfully")
		}
	}
}

func defaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp"
	}
	return filepath.Join(home, ".local", "share", "goes-wallpaper")
}

func isValidSize(size string) bool {
	for _, s := range validSizes {
		if s == size {
			return true
		}
	}
	return false
}

func buildImageURL(size string) string {
	if size == "latest" {
		return baseURL + "latest.jpg"
	}
	return baseURL + size + ".jpg"
}

// cropAndConvert crops the upper-left 2/3 of the source image and converts to PNG.
// This removes the Atlantic Ocean and Mexico from the GOES CONUS imagery and avoids
// GNOME's gdk-pixbuf JPEG rendering issues with large files.
func cropAndConvert(srcPath, destPath string, verbose bool) error {
	if verbose {
		fmt.Printf("Cropping upper-left 2/3 and converting to PNG: %s\n", destPath)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source image: %w", err)
	}
	defer srcFile.Close()

	img, err := jpeg.Decode(srcFile)
	if err != nil {
		return fmt.Errorf("failed to decode JPEG: %w", err)
	}

	bounds := img.Bounds()
	cropWidth := bounds.Dx() * 2 / 3
	cropHeight := bounds.Dy() * 2 / 3
	cropRect := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+cropWidth, bounds.Min.Y+cropHeight)

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	cropped := img.(subImager).SubImage(cropRect)

	// Draw the cropped image into a mutable RGBA so we can overlay the timestamp
	dst := image.NewRGBA(cropped.Bounds())
	draw.Draw(dst, dst.Bounds(), cropped, cropped.Bounds().Min, draw.Src)

	// Render current time and repo URL
	timestamp := time.Now().Format("15:04:05")
	repoURL := "goeswall"
	drawTimestamp(dst, timestamp, repoURL)

	tmpPath := destPath + ".tmp.png"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	if err := png.Encode(outFile, dst); err != nil {
		outFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to encode PNG: %w", err)
	}
	if err := outFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close output file: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to move processed image to final path: %w", err)
	}

	return nil
}

func downloadImage(url, destPath string) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Write to a temp file first, then rename for atomicity
	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = io.Copy(f, resp.Body)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write image: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to move image to final path: %w", err)
	}

	return nil
}

func setWallpaper(imagePath, method string, verbose bool) error {
	absPath, err := filepath.Abs(imagePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if method == "auto" {
		method = detectDesktopEnvironment()
		if verbose {
			fmt.Printf("Detected wallpaper method: %s\n", method)
		}
	}

	switch method {
	case "gnome":
		return setWallpaperGnome(absPath)
	case "kde":
		return setWallpaperKDE(absPath)
	case "xfce":
		return setWallpaperXfce(absPath)
	case "sway":
		return setWallpaperSway(absPath)
	case "feh":
		return setWallpaperFeh(absPath)
	case "nitrogen":
		return setWallpaperNitrogen(absPath)
	default:
		return fmt.Errorf("unsupported wallpaper method: %s", method)
	}
}

func detectDesktopEnvironment() string {
	if runtime.GOOS != "linux" {
		return "feh" // fallback
	}

	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	session := os.Getenv("DESKTOP_SESSION")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")

	desktop = strings.ToLower(desktop)
	session = strings.ToLower(session)

	switch {
	case strings.Contains(desktop, "gnome") || strings.Contains(session, "gnome"):
		return "gnome"
	case strings.Contains(desktop, "kde") || strings.Contains(session, "plasma"):
		return "kde"
	case strings.Contains(desktop, "xfce") || strings.Contains(session, "xfce"):
		return "xfce"
	case strings.Contains(desktop, "sway") || (waylandDisplay != "" && strings.Contains(session, "sway")):
		return "sway"
	default:
		// Try feh as a reasonable fallback for X11 window managers
		if _, err := exec.LookPath("feh"); err == nil {
			return "feh"
		}
		if _, err := exec.LookPath("nitrogen"); err == nil {
			return "nitrogen"
		}
		return "gnome" // last resort fallback
	}
}

func setWallpaperGnome(path string) error {
	uri := "file://" + path
	// Set for both light and dark mode
	if err := runCmd("gsettings", "set", "org.gnome.desktop.background", "picture-uri", uri); err != nil {
		return err
	}
	// Older GNOME versions may not have picture-uri-dark, ignore error
	_ = runCmd("gsettings", "set", "org.gnome.desktop.background", "picture-uri-dark", uri)
	return runCmd("gsettings", "set", "org.gnome.desktop.background", "picture-options", "zoom")
}

func setWallpaperKDE(path string) error {
	script := fmt.Sprintf(`
var allDesktops = desktops();
for (var i = 0; i < allDesktops.length; i++) {
    var d = allDesktops[i];
    d.wallpaperPlugin = "org.kde.image";
    d.currentConfigGroup = Array("Wallpaper", "org.kde.image", "General");
    d.writeConfig("Image", "file://%s");
}
`, path)
	return runCmd("qdbus", "org.kde.plasmashell", "/PlasmaShell", "org.kde.PlasmaShell.evaluateScript", script)
}

func setWallpaperXfce(path string) error {
	return runCmd("xfconf-query", "-c", "xfce4-desktop", "-p",
		"/backdrop/screen0/monitor0/workspace0/last-image", "-s", path)
}

func setWallpaperSway(path string) error {
	return runCmd("swaymsg", "output", "*", "bg", path, "fill")
}

func setWallpaperFeh(path string) error {
	return runCmd("feh", "--bg-fill", path)
}

func setWallpaperNitrogen(path string) error {
	return runCmd("nitrogen", "--set-zoom-fill", path)
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %q failed: %w", name, err)
	}
	return nil
}

// drawTimestamp renders lines of text in light grey at the top-left of the image, scaled 2x.
func drawTimestamp(img *image.RGBA, lines ...string) {
	face := basicfont.Face7x13
	col := color.RGBA{R: 200, G: 200, B: 200, A: 255}

	glyphWidth := 7
	glyphHeight := 13
	descent := 2 // basicfont.Face7x13 descent
	imgHeight := glyphHeight + descent
	scale := 2
	lineSpacing := 4 // pixels between lines at 1x

	bounds := img.Bounds()
	oy := bounds.Min.Y + timestampOffsetY

	for _, text := range lines {
		textWidth := len(text) * glyphWidth

		// Create a temporary image for the text at 1x (include descent for g, p, y, etc.)
		tmp := image.NewRGBA(image.Rect(0, 0, textWidth, imgHeight))
		d := &font.Drawer{
			Dst:  tmp,
			Src:  image.NewUniform(col),
			Face: face,
			Dot:  fixed.P(0, glyphHeight-descent),
		}
		d.DrawString(text)

		// Draw scaled 2x into the destination image
		ox := bounds.Min.X + timestampOffsetX
		for sy := 0; sy < imgHeight; sy++ {
			for sx := 0; sx < textWidth; sx++ {
				c := tmp.RGBAAt(sx, sy)
				if c.A > 0 {
					for dy := 0; dy < scale; dy++ {
						for dx := 0; dx < scale; dx++ {
							img.SetRGBA(ox+sx*scale+dx, oy+sy*scale+dy, c)
						}
					}
				}
			}
		}

		oy += (imgHeight + lineSpacing) * scale
	}
}
