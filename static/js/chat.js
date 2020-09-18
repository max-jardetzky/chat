// chat.js
for (let el of document.querySelectorAll('.main')) el.style.display = 'none'; 
var windowAddr = window.location.href.substring(window.location.href.indexOf("/", window.location.href.indexOf("/") + 1) + 1, window.location.href.length);
var name;
var input;
var output;
var socket;
var subIndex;
var msgCount;

nameInput = document.getElementById("nameInput")
main = document.getElementById("main");
input = document.getElementById("input");
output = document.getElementById("output");
topbar = document.getElementById("topbar");

nameInput.focus();

var ipText = document.createElement("h1");
ipText.setAttribute("id", "ip")
ipText.innerHTML = "IP: " + windowAddr;
loginContainer.insertBefore(ipText, loginContainer.firstChild)

document.getElementById("nameInput").addEventListener("keyup", function(event) {
    event.preventDefault();
    if (event.keyCode === 13 && nameInput.value != "") {
        openSocket();
    }
});

function submitUsername() {
    if (nameInput.value != "") {
        openSocket();
    }
}

function openSocket() {
    name = nameInput.value;
    document.getElementById("loginContainer").remove();
    for (let el of document.querySelectorAll('.main')) el.style.display = 'block';
    main.insertBefore(ipText, main.firstChild)
    input.focus();
    msgCount = 0;

    //socket = new WebSocket("ws://" + windowAddr + ":80/chat");
    //socket = new WebSocket("ws://" + windowAddr + ":80/chat/chat");
    //socket = new WebSocket("ws://10.63.1.244:80/chat/chat");
    socket = new WebSocket("ws://" + windowAddr.substring(0, windowAddr.indexOf("/")) + ":8080/chat/chat");

    socket.onopen = function() {
        socket.send(name);
    };

    socket.onmessage = function(e) {
        display(e.data)
    };

    function display(displayMsg) {
        var msg = document.createElement("p");
        msg.className = "msg";
        msg.innerText = displayMsg;
        
        if (msgCount > 0) {
            output.appendChild(document.createElement("br"));
        }
        output.appendChild(msg);
        output.scrollTop = output.scrollHeight;
        msgCount++;
    }

    document.getElementById("input").addEventListener("keyup", function(event) {
        event.preventDefault();
        if (event.keyCode === 13 && input.value != "") {
            switch(input.value) {
                case "/help":
                    display("Commands: /users, /clear");
                    break;
                case "/users":
                    $.ajax({
                        url: "http://" + windowAddr + "/users",
                        method: "GET",
                        success: function(data) {
                            display(data)
                        },
                    });
                    break;
                case "/clear":
                    output.innerHTML = "";
                    display("Chat cleared.");
                    break;
                default:
                    socket.send(input.value)
            }
            input.value = "";
        }
    });
}
