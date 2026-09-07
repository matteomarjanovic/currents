import numpy as np


def as_array(rows, column):
    return np.stack([row[column].to_numpy() for row in rows])
