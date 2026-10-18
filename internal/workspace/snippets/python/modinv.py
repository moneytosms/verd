# modular inverse of a modulo m (m need not be prime, gcd(a, m) must be 1); Python 3.8+
def modinv(a, m):
    return pow(a, -1, m)
