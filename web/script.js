const ws = new WebSocket("ws://localhost:8080/ws")

const canvas = document.getElementById("board")
const ctx = canvas.getContext("2d")

let drawing = false

canvas.addEventListener("mousedown", () => drawing = true)
canvas.addEventListener("mouseup", () => drawing = false)

canvas.addEventListener("mousemove", draw)

function draw(e) {

    if (!drawing) return

    const data = {
        type: "draw",
        x: e.offsetX,
        y: e.offsetY
    }

    ws.send(JSON.stringify(data))
}

ws.onmessage = (event) => {

    const msg = JSON.parse(event.data)

    if (msg.type === "draw") {

        ctx.lineTo(msg.x, msg.y)
        ctx.stroke()

    }

    if (msg.type === "chat") {

        const li = document.createElement("li")
        li.innerText = msg.data
        document.getElementById("messages").appendChild(li)

    }
}

document.getElementById("chat").addEventListener("keydown", e => {

    if (e.key === "Enter") {

        ws.send(JSON.stringify({
            type: "chat",
            data: e.target.value
        }))

        e.target.value = ""
    }

})