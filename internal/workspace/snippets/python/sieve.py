# sieve of Eratosthenes: is_prime[i] for 0 <= i <= n in O(n log log n)
def sieve(n):
    is_prime = [True] * (n + 1)
    for i in range(min(2, n + 1)):
        is_prime[i] = False
    for i in range(2, int(n**0.5) + 1):
        if is_prime[i]:
            is_prime[i * i :: i] = [False] * len(range(i * i, n + 1, i))
    return is_prime
