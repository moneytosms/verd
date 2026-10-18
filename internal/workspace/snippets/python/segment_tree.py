# iterative segment tree for sums: set(i, v) is a point update, query(l, r) the sum of [l, r); swap + for min/max
class SegTree:
    def __init__(self, n):
        self.n = n
        self.t = [0] * (2 * n)

    def set(self, i, v):
        i += self.n
        self.t[i] = v
        while i > 1:
            self.t[i >> 1] = self.t[i] + self.t[i ^ 1]
            i >>= 1

    def query(self, l, r):
        s = 0
        l += self.n
        r += self.n
        while l < r:
            if l & 1:
                s += self.t[l]
                l += 1
            if r & 1:
                r -= 1
                s += self.t[r]
            l >>= 1
            r >>= 1
        return s
