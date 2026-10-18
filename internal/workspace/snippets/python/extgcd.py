# extended gcd: returns (g, x, y) with a*x + b*y = g = gcd(a, b)
def extgcd(a, b):
    if b == 0:
        return a, 1, 0
    g, x, y = extgcd(b, a % b)
    return g, y, x - a // b * y
