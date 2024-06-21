import flask, sys
from flask import Flask, request, send_file

app=Flask(__name__)

@app.get("/")
def main():
    response=flask.Response("good morning !?!?!?!?")
    response.headers.add("hi", "yo")
    return response

@app.get("/program.zip")
def program_zip():
    return send_file("base.zip")

app.run("0.0.0.0", int(sys.argv[1]))