# Email Verification Endpoint Testing Guide

## Endpoint Details

**URL:** `GET /auth/verify-email`  
**Query Parameter:** `token` (required)

## Test Scenarios

### 1. Successful Email Verification

**Request:**
```bash
curl -X GET "http://localhost:8081/auth/verify-email?token=<valid-token>"
```

**Expected Response (200 OK):**
```json
{
  "success": true,
  "message": "Email verified successfully"
}
```

### 2. Already Verified Email

**Request:**
```bash
curl -X GET "http://localhost:8081/auth/verify-email?token=<already-used-token>"
```

**Expected Response (200 OK):**
```json
{
  "success": true,
  "message": "Email already verified"
}
```

### 3. Invalid Token

**Request:**
```bash
curl -X GET "http://localhost:8081/auth/verify-email?token=invalid-token-123"
```

**Expected Response (404 Not Found):**
```json
{
  "success": false,
  "error": "Invalid verification token"
}
```

### 4. Expired Token

**Request:**
```bash
curl -X GET "http://localhost:8081/auth/verify-email?token=<expired-token>"
```

**Expected Response (400 Bad Request):**
```json
{
  "success": false,
  "error": "Verification token has expired"
}
```

### 5. Missing Token

**Request:**
```bash
curl -X GET "http://localhost:8081/auth/verify-email"
```

**Expected Response (400 Bad Request):**
```json
{
  "success": false,
  "error": "Verification token is required"
}
```

## Integration Testing Steps

1. **Start the Auth Service:**
   ```bash
   cd services/auth-service
   go run main.go
   ```

2. **Register a new user:**
   ```bash
   curl -X POST http://localhost:8081/auth/register \
     -H "Content-Type: application/json" \
     -d '{
       "email": "test@example.com",
       "password": "SecurePass123!",
       "account_name": "Test Account"
     }'
   ```

3. **Extract the verification token from the database:**
   ```sql
   SELECT verification_token, verification_token_expiry, email_verified 
   FROM users 
   WHERE email = 'test@example.com';
   ```

4. **Verify the email using the token:**
   ```bash
   curl -X GET "http://localhost:8081/auth/verify-email?token=<token-from-db>"
   ```

5. **Verify the database was updated:**
   ```sql
   SELECT email_verified, verification_token, verification_token_expiry 
   FROM users 
   WHERE email = 'test@example.com';
   ```
   
   Expected result:
   - `email_verified` = 1 (true)
   - `verification_token` = NULL
   - `verification_token_expiry` = NULL

## Requirements Validation

This implementation satisfies:

- **Requirement 24.4:** Validates verification token and expiry
  - ✅ Queries users table for verification_token
  - ✅ Checks verification_token_expiry against current time
  - ✅ Returns appropriate error for invalid/expired tokens

- **Requirement 24.5:** Sets email_verified=true on success
  - ✅ Updates email_verified to TRUE
  - ✅ Clears verification_token and verification_token_expiry
  - ✅ Returns success response

## Unit Test Coverage

Run unit tests:
```bash
cd services/auth-service
go test -v ./handlers/verify_email_test.go ./handlers/verify_email.go
```

All test cases:
- ✅ TestVerifyEmail_Success
- ✅ TestVerifyEmail_AlreadyVerified
- ✅ TestVerifyEmail_InvalidToken
- ✅ TestVerifyEmail_ExpiredToken
- ✅ TestVerifyEmail_MissingToken
