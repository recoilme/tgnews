# `tgnews`


Experimental app for text classification.


# Getting Started

## Features

* Command line interface for interactive training and clasification
* Isolate articles in English and Russian
* Isolate news articles
* Group news articles by category
* Group similar news into threads
* Sort threads by their relative importance
* Simple, pure Go implementation

## Installing

To start using `tgnews`, install Go 1.13 and run `go build`:

```sh
$ go get -u github.com/recoilme/tgnews
$ go build
```

This will retrieve the app.

## Usage

```
tgnews train source_dir
tgnews languages source_dir
tgnews news source_dir
tgnews categories source_dir
tgnews threads source_dir
tgnews top source_dir
```

## Performance

Perfomance is bad right now (around 1000 docs/min). Wait for optimisations. Take a look at `gonum` and `sparse matrix`


## How it is done

* Get articles by categories from train folder
* Calculate TF/IDF
* Calculate cosine similarity with catgory/article
* Threads weighted by similarity with category

## Limitations

* Pretrained dataset are small

## Contact

Vadim Kulibaba [@recoilme](https://github.com/recoilme)

## License

`tgnews` source code is available under the MIT [License](/LICENSE).