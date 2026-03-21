"""Unit tests for AES-256 encryption/decryption (services/crypto.py).
Requirements: 8.3
"""
import pytest
from services.crypto import encrypt, decrypt


class TestEncryptDecrypt:
    """Tests for encrypt/decrypt round-trip."""

    def test_encrypt_returns_string(self):
        ciphertext = encrypt("hello", "my-secret-key")
        assert isinstance(ciphertext, str)
        assert ciphertext != "hello"

    def test_decrypt_round_trip(self):
        key = "test-aes-key-32-bytes-long-padded"
        plaintext = "super-secret-password"
        ciphertext = encrypt(plaintext, key)
        assert decrypt(ciphertext, key) == plaintext

    def test_different_keys_produce_different_ciphertext(self):
        ct1 = encrypt("password", "key-one")
        ct2 = encrypt("password", "key-two")
        assert ct1 != ct2

    def test_decrypt_with_wrong_key_raises(self):
        ciphertext = encrypt("secret", "correct-key")
        with pytest.raises(Exception):
            decrypt(ciphertext, "wrong-key")

    def test_empty_string_round_trip(self):
        key = "any-key"
        assert decrypt(encrypt("", key), key) == ""

    def test_unicode_round_trip(self):
        key = "unicode-test-key"
        plaintext = "pässwörd-with-ünïcödé"
        assert decrypt(encrypt(plaintext, key), key) == plaintext

    def test_long_string_round_trip(self):
        key = "long-string-key"
        plaintext = "x" * 10000
        assert decrypt(encrypt(plaintext, key), key) == plaintext
