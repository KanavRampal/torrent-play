# Torrent to HLS Server

## 🧭 Index

- [Description](#description)
- [How to Run Locally](#how-to-run-locally)
- [API Overview](#api-overview)
- [License](#license)

---

## 📄 Description

This project is a Go-based server that exposes an HTTP API to add magnet links and generate consumable HLS (HTTP Live Streaming) URLs on the fly. It works by:

1. Accepting a magnet URI via a REST API.
2. Downloading the associated torrent content using a torrent client.
3. Launching an `ffmpeg` process to transcode the video file into HLS format in real-time.
4. Serving the generated `.m3u8` playlist and video segments over HTTP.

This tool can be used to stream torrent-based media via any HLS-compatible player with minimal delay and no manual setup.

---

## 🚀 How to Run Locally

### 🧱 Requirements

- Go 1.20+
- `ffmpeg` installed and accessible in the system path
- Git (for cloning the repo)
- Optional: `vlc`, `mpv`, or any HLS-capable player to test the output

### 📥 Clone the Repository

```bash
git clone https://github.com/yourusername/torrent-to-hls-server.git
cd torrent-to-hls-server
````

### 🛠 Build the Server

```bash
go build -o torrent-hls-server main.go
```

### ⚙️ Run the Server

```bash
./torrent-hls-server
```

The server should now be running on `http://localhost:8080`.

---

## 🧪 API Overview

### `POST /api/add`

Add a new magnet link to start downloading and transcoding.

**Request:**

```json
{
  "magnet": "magnet:?xt=urn:btih:..."
}
```

**Response:**

```json
{
  "status": "started",
  "hls_url": "http://localhost:8080/stream/<id>/index.m3u8"
}
```


