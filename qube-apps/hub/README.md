# Qube Hub

A Docker Hub-like web application for managing and sharing Qube container images.

## Features

- 🔐 **User Authentication**: Register, login, and manage your account
- 📦 **Image Management**: Upload, download, and manage Qube container images
- 🔍 **Search & Discovery**: Search and explore public container images
- 💾 **ScyllaDB Backend**: High-performance distributed database
- 🚀 **REST API**: Full-featured API for programmatic access
- 🎨 **Modern UI**: Clean and responsive web interface

## Architecture

### Backend
- **Gin Web Framework**: High-performance HTTP server
- **ScyllaDB**: Distributed NoSQL database for scalability
- **JWT Authentication**: Secure token-based authentication
- **File Storage**: Local filesystem storage for container images

### Frontend
- **Vanilla JavaScript**: No framework dependencies
- **Responsive Design**: Works on desktop and mobile
- **Real-time Updates**: Dynamic image loading and search

## Installation

### Prerequisites

- Go 1.21+
- ScyllaDB (or Cassandra)

### Setup

1. Clone the repository:
```bash
cd hub
```

2. Copy the example environment file:
```bash
cp .env.example .env
```

3. Edit `.env` with your configuration:
```env
ADDR=:2112
SCYLLA_HOSTS=192.168.1.87
SCYLLA_KEYSPACE=qube_hub
JWT_SECRET=your-secret-key-here
```

4. Install dependencies:
```bash
go mod download
```

5. Run the server:
```bash
go run main.go
```

The server will be available at `http://localhost:2112`

## API Endpoints

### Authentication

- `POST /api/auth/register` - Register a new user
- `POST /api/auth/login` - Login and get JWT token
- `GET /api/auth/profile` - Get current user profile (requires auth)

### Images

- `GET /api/images` - List all public images
- `GET /api/images/:name` - Get all versions of an image
- `GET /api/images/:name/:tag/download` - Download an image
- `POST /api/images/upload` - Upload a new image (requires auth)
- `GET /api/images/my` - Get your images (requires auth)
- `DELETE /api/images/:id` - Delete an image (requires auth)

### Example API Usage

**Register:**
```bash
curl -X POST http://localhost:2112/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john",
    "email": "john@example.com",
    "password": "password123"
  }'
```

**Login:**
```bash
curl -X POST http://localhost:2112/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john",
    "password": "password123"
  }'
```

**Upload Image:**
```bash
curl -X POST http://localhost:2112/api/images/upload \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "name=alpine" \
  -F "tag=latest" \
  -F "description=Alpine Linux base image" \
  -F "is_public=true" \
  -F "file=@alpine.tar"
```

**Download Image:**
```bash
curl -O http://localhost:2112/api/images/alpine/latest/download
```

## Database Schema

### Users Table
- `id` (UUID) - Primary key
- `username` (TEXT) - Unique username
- `email` (TEXT) - Email address
- `password_hash` (TEXT) - Bcrypt hashed password
- `created_at` (TIMESTAMP)
- `updated_at` (TIMESTAMP)

### Images Table
- `id` (UUID) - Primary key
- `name` (TEXT) - Image name
- `tag` (TEXT) - Image tag/version
- `owner_id` (UUID) - User who uploaded
- `description` (TEXT) - Description
- `size` (BIGINT) - File size in bytes
- `downloads` (COUNTER) - Download count
- `is_public` (BOOLEAN) - Public visibility
- `file_path` (TEXT) - Storage path
- `created_at` (TIMESTAMP)
- `updated_at` (TIMESTAMP)

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ADDR` | Server address | `:2112` |
| `DEBUG` | Debug mode | `false` |
| `SCYLLA_HOSTS` | ScyllaDB hosts (comma-separated) | `127.0.0.1` |
| `SCYLLA_KEYSPACE` | Database keyspace | `qube_hub` |
| `SCYLLA_USERNAME` | Database username | `""` |
| `SCYLLA_PASSWORD` | Database password | `""` |
| `JWT_SECRET` | JWT signing secret | Required |
| `STORAGE_PATH` | File storage directory | `./storage/images` |
| `MAX_UPLOAD_SIZE` | Max upload size in bytes | `1073741824` (1GB) |

## Integration with Qube CLI

The hub is compatible with the existing Qube CLI. Update your Qube configuration to point to your hub:

```bash
# In internal/config/config.go or via environment
export QUBE_HUB_URL="http://localhost:2112"
```

Images can then be pulled using:
```bash
qube pull imagename:tag
```

## Development

### Project Structure

```
hub/
├── main.go                 # Application entry point
├── internal/
│   ├── config/            # Configuration management
│   ├── database/          # ScyllaDB connection
│   ├── handlers/          # HTTP handlers
│   ├── middleware/        # Middleware (auth, etc.)
│   ├── models/            # Data models
│   └── router/            # Route definitions
├── static/                # Static assets
│   ├── css/
│   └── js/
├── templates/             # HTML templates
└── storage/               # File storage (created at runtime)
```

### Building

```bash
go build -o qube-hub main.go
```

### Running in Production

```bash
# Set production environment
export DEBUG=false
export JWT_SECRET=strong-random-secret-here

# Run with systemd or supervisor
./qube-hub
```

## Security Considerations

1. **Change JWT Secret**: Use a strong, random secret in production
2. **HTTPS**: Use a reverse proxy (nginx/caddy) with TLS
3. **File Validation**: Only .tar files are accepted
4. **Rate Limiting**: Consider adding rate limiting middleware
5. **File Size Limits**: Adjust `MAX_UPLOAD_SIZE` based on your needs

## License

Same as the main Qube project

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
