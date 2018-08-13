// chat.js
for (let el of document.querySelectorAll('.main')) el.style.display = 'none'; 
var windowAddr = window.location.href.substring(window.location.href.indexOf("/", window.location.href.indexOf("/") + 1) + 1, window.location.href.length - 1);
var name;
var input;
var output;
var socket;
var subIndex;

input = document.getElementById("input");
output = document.getElementById("output");

nameInput.focus();

document.getElementById("nameInput").addEventListener("keyup", function(event) {
    event.preventDefault();
    if (event.keyCode === 13 && nameInput.value != "") {
        openSocket();
    }
});

function openSocket() {
    name = nameInput.value;
    document.getElementById("login").remove();
    for (let el of document.querySelectorAll('.main')) el.style.display = 'block';
    input.focus();

    socket = new WebSocket("ws://" + windowAddr + ":80/chat");

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
        
        output.appendChild(msg)
        output.appendChild(document.createElement("br"))
        output.scrollTop = output.scrollHeight;
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