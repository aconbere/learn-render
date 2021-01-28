main:
	go build -tags netgo -ldflags '-s -w' -o main

clean:
	rm main
