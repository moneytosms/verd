# coordinate compression: returns a with each value replaced by its rank among the distinct values
def compress(a):
    rank = {v: i for i, v in enumerate(sorted(set(a)))}
    return [rank[x] for x in a]
