# Prints one random input. argv[1] is the seed; $SIZE grows from 1 to 10 so early failures are small.
import os
import random
import sys

rng = random.Random(int(sys.argv[1]))
size = int(os.getenv("SIZE", "10"))
n = rng.randint(1, size)
print(n)
print(*[rng.randint(1, size) for _ in range(n)])
# tree: for v in range(2, n + 1): print(rng.randint(1, v - 1), v)
# string: print("".join(rng.choice("abc") for _ in range(n)))
