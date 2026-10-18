# Fenwick tree (BIT), 0-indexed: add(i, v) is a point update, sum(l, r) is the sum of [l, r); O(log n)
class Fenwick:
    def __init__(self, n):
        self.n = n
        self.t = [0] * (n + 1)

    def add(self, i, v):
        i += 1
        while i <= self.n:
            self.t[i] += v
            i += i & -i

    def prefix(self, i):
        s = 0
        while i > 0:
            s += self.t[i]
            i -= i & -i
        return s

    def sum(self, l, r):
        return self.prefix(r) - self.prefix(l)
