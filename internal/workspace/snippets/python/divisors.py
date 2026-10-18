# all divisors of n in increasing order, O(sqrt n)
def divisors(n):
    small, large = [], []
    d = 1
    while d * d <= n:
        if n % d == 0:
            small.append(d)
            if d != n // d:
                large.append(n // d)
        d += 1
    return small + large[::-1]
