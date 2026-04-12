# GoSource-Insight API

This is the backend API for GoSource-Insight, providing user authentication, project management, and code analysis functionality.

## Features

- JWT-based user authentication
- Project management
- Code analysis
- User management
- RESTful API endpoints

## Tech Stack

- Go 1.24.4
- Gin web framework
- PostgreSQL database
- JWT for authentication
- GORM for ORM

## API Endpoints

### Authentication
- `POST /api/v1/users/register` - Register a new user
- `POST /api/v1/users/login` - Login user and get JWT token
- `GET /api/v1/users/profile` - Get user profile (requires authentication)

### Projects
- `POST /api/v1/projects` - Create a new project (requires authentication)
- `GET /api/v1/projects` - List all projects (requires authentication)
- `GET /api/v1/projects/:id` - Get project details (requires authentication)
- `DELETE /api/v1/projects/:id` - Delete a project (requires authentication)

### Analysis
- `POST /api/v1/analysis/analyze` - Analyze code (requires authentication)
- `GET /api/v1/analysis/:projectId` - Get analysis results (requires authentication)

## Setup

1. Install dependencies: `go mod download`
2. Set up environment variables:
   ```
   PORT=8080
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=yourpassword
   DB_NAME=goaiinsight
   JWT_SECRET=your-secret-key
   ```
3. Run the server: `go run main.go`

## JWT Authentication

The API uses JWT (JSON Web Tokens) for authentication. All protected endpoints require a valid JWT token in the Authorization header:

```
Authorization: Bearer <your-jwt-token>
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

MIT License - see LICENSE file for details.