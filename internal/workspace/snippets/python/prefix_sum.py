# prefix sums of a: p[i] = a[0] + ... + a[i-1]; sum of a[l:r] is p[r] - p[l]
from itertools import accumulate

p = [0] + list(accumulate(a))
