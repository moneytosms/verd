# square matrix multiply and power modulo mod: O(k^3 log e)
def mat_mul(a, b, mod):
    k = len(a)
    c = [[0] * k for _ in range(k)]
    for i in range(k):
        for l in range(k):
            x = a[i][l]
            if x:
                for j in range(k):
                    c[i][j] = (c[i][j] + x * b[l][j]) % mod
    return c


def mat_pow(a, e, mod):
    k = len(a)
    r = [[int(i == j) for j in range(k)] for i in range(k)]
    while e > 0:
        if e & 1:
            r = mat_mul(r, a, mod)
        a = mat_mul(a, a, mod)
        e >>= 1
    return r
