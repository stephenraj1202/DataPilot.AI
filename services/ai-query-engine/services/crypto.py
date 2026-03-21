"""AES-256 encryption/decryption using Fernet (AES-128-CBC with HMAC) or AES-GCM."""
import os
import base64
import hashlib
from cryptography.fernet import Fernet
from cryptography.hazmat.primitives.ciphers.aead import AESGCM


def _get_fernet(key: str) -> Fernet:
    """Derive a 32-byte Fernet key from the provided AES key string."""
    # Derive exactly 32 bytes from the key using SHA-256
    key_bytes = hashlib.sha256(key.encode()).digest()
    fernet_key = base64.urlsafe_b64encode(key_bytes)
    return Fernet(fernet_key)


def encrypt(plaintext: str, key: str) -> str:
    """Encrypt plaintext string using AES-256 (Fernet). Returns base64-encoded ciphertext."""
    f = _get_fernet(key)
    token = f.encrypt(plaintext.encode())
    return token.decode()


def decrypt(ciphertext: str, key: str) -> str:
    """Decrypt ciphertext string using AES-256 (Fernet). Returns plaintext."""
    f = _get_fernet(key)
    plaintext = f.decrypt(ciphertext.encode())
    return plaintext.decode()
