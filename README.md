# brhttp: A Powerful Static Server in Go with Live Reload 🚀

[![Latest Release](https://img.shields.io/github/v/release/SUKUNA456/brhttp)](https://github.com/SUKUNA456/brhttp/releases)  
[![Open Issues](https://img.shields.io/github/issues/SUKUNA456/brhttp)](https://github.com/SUKUNA456/brhttp/issues)  
[![License](https://img.shields.io/github/license/SUKUNA456/brhttp)](https://github.com/SUKUNA456/brhttp/blob/main/LICENSE)

## Overview

brhttp is a powerful static server built in Go. It focuses on simplicity and performance, making it an excellent choice for developers looking for a minimalistic solution. With features like Live Reload, build automation via webhooks, reverse proxy capabilities, and a comprehensive control API, brhttp streamlines the development process.

## Features

- **Live Reload**: Automatically refresh your browser when files change.
- **Build Automation**: Integrate with webhooks for seamless deployments.
- **Reverse Proxy**: Route requests to different services easily.
- **API Control**: Manage your server with a full-featured API.
- **Minimalist Design**: No unnecessary dependencies, keeping it lightweight.
- **Open Source**: Free to use and modify as per your needs.
- **High Performance**: Optimized for speed and efficiency.

## Installation

To get started with brhttp, download the latest release from the [Releases section](https://github.com/SUKUNA456/brhttp/releases). Choose the appropriate binary for your operating system and architecture, then execute it.

### Example for Linux

```bash
wget https://github.com/SUKUNA456/brhttp/releases/download/v1.0.0/brhttp-linux-amd64
chmod +x brhttp-linux-amd64
./brhttp-linux-amd64
```

### Example for macOS

```bash
curl -LO https://github.com/SUKUNA456/brhttp/releases/download/v1.0.0/brhttp-darwin-amd64
chmod +x brhttp-darwin-amd64
./brhttp-darwin-amd64
```

### Example for Windows

Download the binary from the [Releases section](https://github.com/SUKUNA456/brhttp/releases) and run it directly.

## Usage

Once you have the server running, you can access it at `http://localhost:8080` by default. You can customize the port and other settings through command-line flags.

### Command-Line Options

- `-port`: Specify the port for the server to listen on.
- `-root`: Set the root directory for static files.
- `-enable-reload`: Enable live reload functionality.

### Example Command

```bash
./brhttp-linux-amd64 -port 3000 -root ./public -enable-reload
```

## Configuration

brhttp allows for flexible configuration. You can set up a configuration file to manage your settings more conveniently.

### Example Configuration File

```json
{
  "port": 3000,
  "root": "./public",
  "enable_reload": true,
  "proxy": {
    "enabled": true,
    "target": "http://localhost:4000"
  }
}
```

## API Reference

brhttp comes with a RESTful API that allows you to control the server programmatically. You can start, stop, and configure the server through simple HTTP requests.

### Example API Endpoints

- **Start Server**: `POST /api/start`
- **Stop Server**: `POST /api/stop`
- **Get Status**: `GET /api/status`

### Example cURL Command

```bash
curl -X POST http://localhost:8080/api/start
```

## Topics Covered

- **Automation**: Integrate brhttp into your CI/CD pipeline.
- **Binary**: Standalone binary for easy deployment.
- **Debian**: Compatible with Debian-based systems.
- **Dev Server**: Ideal for development environments.
- **Frontend**: Perfect for serving static frontend applications.
- **Go**: Built with Go for high performance.
- **Golang**: Leverage the power of Golang.
- **Linux**: Fully functional on Linux systems.
- **Live Reload**: Enhance development workflow.
- **Minimalist**: No bloat, just what you need.
- **No Dependencies**: Simple setup without extra libraries.
- **Open Source**: Contribute to the project on GitHub.
- **Performance**: Fast and efficient static file serving.
- **Reverse Proxy**: Manage multiple services easily.
- **Security**: Built with best practices in mind.
- **SPA**: Serve Single Page Applications seamlessly.
- **Static Server**: Designed for serving static content.
- **Zero Config**: Get started with minimal setup.

## Contribution

We welcome contributions to brhttp! If you have ideas, improvements, or bug fixes, please open an issue or submit a pull request. 

### How to Contribute

1. Fork the repository.
2. Create a new branch.
3. Make your changes.
4. Commit and push your changes.
5. Open a pull request.

## License

brhttp is open-source software licensed under the MIT License. You can view the full license [here](https://github.com/SUKUNA456/brhttp/blob/main/LICENSE).

## Support

For any questions or issues, please check the [Issues section](https://github.com/SUKUNA456/brhttp/issues) or open a new issue if you need help.

## Resources

- [Go Documentation](https://golang.org/doc/)
- [GitHub Actions](https://docs.github.com/en/actions)
- [Webhooks](https://developer.github.com/webhooks/)
- [Reverse Proxy Basics](https://www.nginx.com/resources/glossary/reverse-proxy-server/)

## Acknowledgments

Thank you to all contributors and users who make brhttp better. Your feedback and contributions are invaluable.

For more information, visit the [Releases section](https://github.com/SUKUNA456/brhttp/releases) to download the latest version and stay updated with new features and improvements.