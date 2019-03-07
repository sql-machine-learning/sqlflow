import sql_data
import unittest


class TestSQLData(unittest.TestCase):
    def __init__(self, *args, **kwargs):
        super(TestSQLData, self).__init__(*args, **kwargs)
        self.db = sql_data.connect('root', 'root', 'localhost', 3306)
        self.assertIsNotNone(self.db)

    def test_load(self):
        f, l = sql_data.load(
            self.db, 'SELECT * FROM iris.train LIMIT 3', 'class', None)
        self.assertEqual(4, len(f.keys()))  # 4 features
        self.assertEqual(3, len(l))  # label column length

    def test_load_with_filter(self):
        fs = ['sepal_length', 'petal_width']
        f, l = sql_data.load(
            self.db, 'SELECT * FROM iris.train LIMIT 3', 'class', fs)
        self.assertEqual(len(fs), len(f))
        self.assertEqual(3, len(l))  # label column length

    def test_feature_columns(self):
        f, l = sql_data.load(
            self.db, 'SELECT * FROM iris.train LIMIT 3', 'class', None)
        c = sql_data.feature_columns(f)
        self.assertEqual(4, len(c))  # 4 features


if __name__ == '__main__':
    unittest.main()
