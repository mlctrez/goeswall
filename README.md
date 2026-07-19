# goeswall

A Go application that downloads the latest GOES-East (GOES-19) satellite imagery of the Continental US (CONUS) and sets it as your desktop wallpaper. Designed to be called from a cron job.

The imagery is the GeoColor product from NOAA's GOES-19 satellite, which shows a natural-looking view during the day and infrared at night.

## Install

```bash
make install
```

This builds the binary and installs it to `~/.local/bin/goeswall`.

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Compile the binary locally |
| `make install` | Build and install to `~/.local/bin/goeswall` |
| `make clean` | Remove the local binary |

## Dependencies

- [ImageMagick](https://imagemagick.org/) (`convert`) — used to crop and convert the downloaded image to PNG

## Usage

```bash
goeswall [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-output` | `~/.local/share/goes-wallpaper` | Directory to save the downloaded image |
| `-size` | `5000x3000` | Image resolution (see sizes below) |
| `-set-wallpaper` | `true` | Whether to set the image as desktop wallpaper |
| `-method` | `auto` | Wallpaper method: auto, gnome, kde, xfce, sway, feh, nitrogen |
| `-verbose` | `false` | Enable verbose output |

### Available Sizes

- `latest` — Default latest.jpg (~1250x750)
- `625x375` — Small
- `1250x750` — Medium
- `2500x1500` — Large
- `5000x3000` — Full resolution (default)
- `10000x6000` — Ultra high resolution

## Cron Setup

To update your wallpaper every 10 minutes:

```bash
crontab -e
```

Add one of these lines depending on your desktop environment:

### GNOME (Ubuntu, Fedora, etc.)

```cron
*/10 * * * * DISPLAY=:0 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u)/bus ~/.local/bin/goeswall -method gnome
```

### Sway / Wayland

```cron
*/10 * * * * SWAYSOCK=$(ls /run/user/$(id -u)/sway-ipc.* 2>/dev/null | head -1) ~/.local/bin/goeswall -method sway
```

### feh (i3, bspwm, dwm, etc.)

```cron
*/10 * * * * DISPLAY=:0 ~/.local/bin/goeswall -method feh
```

### KDE Plasma

```cron
*/10 * * * * DISPLAY=:0 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u)/bus ~/.local/bin/goeswall -method kde
```

### Download only (no wallpaper set)

```cron
*/10 * * * * ~/.local/bin/goeswall -set-wallpaper=false -output /path/to/images
```

## Notes

- Images are updated every 5-10 minutes on the NOAA CDN
- The GeoColor product looks best during daytime hours for your timezone
- The download is atomic (writes to a temp file, then renames) so your wallpaper won't flicker with a partial image
- The image is cropped to the upper-left 2/3 (continental US) and converted to PNG to avoid GNOME rendering issues with large JPEGs
- Image source: NOAA/NESDIS GOES-19 ABI CONUS GeoColor via `cdn.star.nesdis.noaa.gov`
