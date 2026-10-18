# polynomial string hash modulo 2^61 - 1 with a random base; get(l, r) hashes s[l:r] in O(1)
import random

M = (1 << 61) - 1
B = random.randrange(500, M - 500)


class Hash:
    def __init__(self, s):
        self.h = [0]
        self.pw = [1]
        for ch in s:
            self.h.append((self.h[-1] * B + ord(ch)) % M)
            self.pw.append(self.pw[-1] * B % M)

    def get(self, l, r):
        return (self.h[r] - self.h[l] * self.pw[r - l]) % M
