import flask
from flask import Flask, request

app=Flask(__name__)

@app.get("/")
def main():
    response=flask.Response("good morning")
    response.headers.add("hi", "yo")
    return response

app.run("0.0.0.0", 8080)