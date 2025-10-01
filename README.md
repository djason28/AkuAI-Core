# ⚡ AkuAI Backend (Core)

High-performance AI Chat API server built with Go, Gin, and GORM. Features real-time WebSocket chat, user authentication, file uploads, and intelligent response caching.

## 🚀 Tech Stack

- **Language**: Go 1.21+
- **Framework**: Gin (HTTP router)
- **Database**: MySQL with GORM ORM
- **AI Integration**: Google Gemini API
- **Real-time**: WebSocket connections
- **Cache**: In-memory with TTL
- **Authentication**: JWT tokens
- **File Storage**: Local file system

## 📁 Project Structure

```
├── main.go                 # Application entry point
├── go.mod                  # Go module dependencies
├── go.sum                  # Dependency checksums
├── controllers/            # HTTP handlers
│   ├── auth.go            # Authentication endpoints
│   ├── conversation.go    # Chat conversation handlers  
│   ├── profile.go         # User profile management
│   └── ws.go             # WebSocket chat handler
├── middleware/            # HTTP middleware
│   ├── auth.go           # JWT authentication
│   └── ratelimit.go      # Rate limiting
├── models/               # Database models
│   ├── user.go           # User model
│   ├── conversation.go   # Conversation model
│   └── message.go        # Message model
├── routes/               # Route definitions
│   ├── routes.go         # Main route registration
│   ├── auth/             # Auth routes
│   ├── conversation/     # Chat routes
│   ├── profile/          # Profile routes
│   ├── uploads/          # Static file routes
│   └── websocket/        # WebSocket routes
├── pkg/                  # Packages & utilities
│   ├── cache/            # Response caching system
│   ├── config/           # Configuration management
│   ├── services/         # External service integration
│   ├── token/            # JWT token handling
│   └── utills/           # Utility functions
└── uploads/              # Uploaded files directory
```

## 🎯 Features

### 🔐 **Authentication System**
- User registration with validation
- JWT-based authentication
- Secure password hashing (bcrypt)
- Protected route middleware
- Session management

### 💬 **AI Chat System** 
- Real-time WebSocket chat
- Google Gemini API integration
- Streaming response support
- Message history persistence
- Conversation management
- Smart response caching

### 👤 **Profile Management**
- User profile CRUD operations
- Profile image upload/delete
- Image processing and storage
- Secure file handling

### ⚡ **Performance Features**
- Intelligent response caching with TTL
- Rate limiting middleware
- Optimized database queries
- Efficient WebSocket handling
- Background conversation cleanup

### 🛡️ **Security Features**
- CORS protection
- Rate limiting per IP
- JWT token validation
- File upload security
- SQL injection prevention (GORM)

## 🛠️ Development

### Prerequisites
- Go 1.21 or higher
- SQLite3
- Git

### Installation & Setup

```bash
# Clone and navigate to core directory
cd core

# Install dependencies
go mod download

# Run the application
go run main.go

# Or build and run
go build -o AkuAI.exe
./AkuAI.exe
```

### Development with Auto-Reload
```bash
# Using CompileDaemon for auto-reload during development
CompileDaemon --build="go build -o .\bin\AkuAI.exe ." --command=".\\bin\\AkuAI.exe" --pattern="\.go$" --exclude-dir=bin,vendor
```

### Environment Configuration

#### Step 1: Setup Environment File
```bash
# Navigate to core directory
cd core

# Rename .env.example to .env
ren .env.example .env     # Windows
# mv .env.example .env    # Linux/Mac
```

#### Step 2: Generate JWT Secret Key
```bash
# Using PowerShell (Windows)
-join ((1..64) | ForEach-Object { [char]((65..90) + (97..122) + (48..57) | Get-Random) })

# Using OpenSSL (if available)  
openssl rand -base64 64

# Manual - Use any random 32+ character string
```

#### Step 3: Get Gemini API Key

1. **Visit Google AI Studio**: [https://aistudio.google.com](https://aistudio.google.com)
2. **Sign In** with your Google account
3. **Get API Key**: Click "Get API Key" → Create/select project → Copy key

#### Step 4: Fill Required Configuration
Open the `.env` file and update the following values:
```bash
# Required Keys
JWT_SECRET=your_generated_jwt_secret_here
GEMINI_API_KEY=your_copied_gemini_api_key_here
```

#### Step 5: Setup MySQL Database
Before running the application, ensure MySQL is running and create the database:
```sql
CREATE DATABASE AkuAI CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

Or using MySQL command line:
```bash
mysql -u root -p
CREATE DATABASE AkuAI CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
EXIT;
```

#### Important Notes
- Keep your API keys secure and never commit them to version control
- The `.env` file is already included in `.gitignore`
- For production, set environment variables directly on your hosting platform

### Database Migration

The application automatically handles database migration on startup:
- Connects to MySQL database "AkuAI"
- Migrates all models (User, Conversation, Message)
- Sets up necessary indexes and constraints
- Creates tables with proper UTF8MB4 charset for emoji support

## 🔌 API Endpoints

### Authentication
```
POST /register        # User registration
POST /login          # User login  
POST /logout         # User logout (protected)
```

### Profile Management
```
GET    /profile           # Get user profile (protected)
PUT    /profile           # Update user profile (protected)
POST   /profile/image/token    # Get upload token (protected)
POST   /profile/image/upload   # Upload profile image (protected)
GET    /profile/image          # Get profile image URL (protected)
DELETE /profile/image          # Delete profile image (protected)
```

### Chat & Conversations
```
GET    /conversations     # Get user conversations (protected)
POST   /conversations     # Create new conversation (protected)
GET    /conversations/:id # Get conversation messages (protected)
DELETE /conversations/:id # Delete conversation (protected)
DELETE /conversations     # Delete all conversations (protected)
```

### WebSocket
```
GET /ws/chat             # WebSocket chat endpoint (rate limited)
```

### Static Files
```
GET /uploads/*           # Serve uploaded files
```

## 🏗️ Architecture Patterns

### Modular Route Structure
```go
// All routes follow consistent pattern
uploadsRoutes.Register(r, db)
websocketRoutes.Register(r, db) 
authRoutes.RegisterPublic(r, db)
profileRoutes.Register(protected, db)
convRoutes.Register(protected, db)
```

### Smart Caching System
```go
// Cache with status tracking and TTL
type CachedResponse struct {
    Text      string              `json:"text"`
    Status    ResponseStatus      `json:"status"`
    Timestamp time.Time          `json:"timestamp"`
}

// Only cache completed, successful responses
SetChatResponse(key, text, StatusCompleted, 5*time.Minute)
```

### Middleware Chain
```go
// Rate limiting + Authentication
r.GET("/ws/chat", middleware.RateLimit(), controllers.ChatWS(db))

protected := r.Group("/")
protected.Use(middleware.AuthMiddleware())
```

## 🔧 Configuration

### JWT Configuration
```go
// Token settings
const TokenExpiry = 24 * time.Hour
const RefreshThreshold = 1 * time.Hour
```

### Cache Configuration  
```go
// Response cache settings
const DefaultTTL = 5 * time.Minute
const CleanupInterval = 10 * time.Minute
```

### Rate Limiting
```go
// WebSocket rate limiting
const RequestsPerMinute = 30
const BurstLimit = 10
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/cache/...
```

### Test Coverage
- Unit tests for cache system
- Rate limiting tests  
- Authentication middleware tests
- Utility function tests

## 📊 Performance Monitoring

### Logging Features
- Request/response logging
- Cache hit/miss tracking
- WebSocket connection monitoring
- Error tracking and alerting
- Performance metrics

### Cache Analytics
```
Cache HIT: key=abc...xyz, status=completed, text_length=250, cached_at=14:30:15
Cache SAVED: key=abc...xyz, status=completed, text_length=250, ttl=5m0s
Cache INVALIDATED: key=abc...xyz (canceled/failed request)
```

## 🚀 Production Deployment

### Build for Production
```bash
# Build optimized binary
go build -ldflags="-s -w" -o AkuAI

# Or build for different platforms
GOOS=linux GOARCH=amd64 go build -o AkuAI-linux
GOOS=windows GOARCH=amd64 go build -o AkuAI.exe
```

### Production Configuration
- Set production JWT secret
- Configure CORS for frontend domain
- Set up reverse proxy (nginx/Apache)
- Configure SSL/TLS certificates
- Set up log rotation
- Configure database backup

### Docker Support (Optional)
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o AkuAI

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/AkuAI .
EXPOSE 5000
CMD ["./AkuAI"]
```

## 🔗 Related

- **Frontend**: See `../views/README.md` for SvelteKit client
- **API Documentation**: Available at `/docs` endpoint (if enabled)
- **Database Schema**: See `/models` for GORM model definitions