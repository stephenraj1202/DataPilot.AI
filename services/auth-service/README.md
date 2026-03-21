# Auth Service

The Auth Service handles user authentication, registration, and authorization for the FinOps Platform.

## Features

- User registration with email verification
- Password validation (min 12 chars, uppercase, lowercase, number, special char)
- Common password rejection (top 10,000 passwords)
- Bcrypt password hashing (cost factor 12)
- Email verification with 24-hour token expiry
- SMTP email sending

## API Endpoints

### POST /auth/register

Register a new user account.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "account_name": "Acme Corp"
}
```

**Response (201 Created):**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "verification_sent": true
}
```

**Error Responses:**

- `400 Bad Request` - Invalid request format or password requirements not met
- `409 Conflict` - Email already registered
- `500 Internal Server Error` - Server error

**Password Requirements:**
- Minimum 12 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one number
- At least one special character (!@#$%^&*()_+-=[]{}; ':"\\|,.<>/?)
- Not in top 10,000 common passwords list

## Running the Service

### Prerequisites

- Go 1.18 or higher
- MySQL database
- SMTP server (for email sending)

### Configuration

Update `../../config.ini` with your settings:

```ini
[database]
host=localhost
port=3306
username=root
password=rootpassword
database_name=finops_platform

[mail]
default_smtp_host=smtp.gmail.com
default_smtp_port=587
default_from_email=noreply@finops-platform.com
super_admin_email=admin@finops-platform.com
```

### Start the Service

```bash
cd services/auth-service
go run main.go
```

The service will start on port 8081.

### Health Check

```bash
curl http://localhost:8081/health
```

## Testing

### Run Unit Tests

```bash
go test ./handlers -v
```

### Test Registration Endpoint

```bash
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "SecurePass123!",
    "account_name": "Test Account"
  }'
```

## Architecture

### Components

- **RegisterHandler**: Handles user registration requests
- **EmailService**: Sends verification emails via SMTP
- **Password Validation**: Enforces strong password requirements
- **Common Password Check**: Rejects commonly used passwords

### Database Tables

- **accounts**: Stores account information
- **users**: Stores user credentials and verification status

### Security Features

- Bcrypt password hashing with cost factor 12
- Cryptographically secure token generation
- Email verification required before account activation
- Common password rejection
- Transaction-based account/user creation

## Development

### Project Structure

```
services/auth-service/
├── handlers/
│   ├── register.go           # Registration handler
│   ├── register_test.go      # Unit tests
│   └── common_passwords.go   # Common passwords list
├── utils/
│   └── email.go              # Email sending utility
├── main.go                   # Service entry point
├── go.mod                    # Go dependencies
└── README.md                 # This file
```

### Adding New Endpoints

1. Create a new handler in `handlers/` directory
2. Add tests in corresponding `*_test.go` file
3. Register the route in `main.go`
4. Update this README with endpoint documentation

## Future Enhancements

- [ ] JWT token generation and validation
- [ ] Password reset flow
- [ ] API key management
- [ ] OAuth integration
- [ ] Rate limiting
- [ ] Audit logging
