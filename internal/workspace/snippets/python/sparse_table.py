# sparse table for range minimum on a static array: O(n log n) build, O(1) query of [l, r)
class SparseTable:
    def __init__(self, a):
        n = len(a)
        self.t = [list(a)]
        j = 1
        while (1 << j) <= n:
            prev = self.t[-1]
            half = 1 << (j - 1)
            self.t.append([min(prev[i], prev[i + half]) for i in range(n - (1 << j) + 1)])
            j += 1

    def query(self, l, r):
        j = (r - l).bit_length() - 1
        return min(self.t[j][l], self.t[j][r - (1 << j)])
