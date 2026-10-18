// modular inverse of a modulo a prime p (Fermat); use extgcd when p is not prime
long long modinv(long long a, long long p) {
    long long r = 1, e = p - 2;
    for (a %= p; e > 0; e >>= 1, a = a * a % p)
        if (e & 1) r = r * a % p;
    return r;
}
