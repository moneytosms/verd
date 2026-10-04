# Prints one random input. argv[1] is the seed.
import random
import sys

rng = random.Random(int(sys.argv[1]))
print(rng.randint(1, 10))
