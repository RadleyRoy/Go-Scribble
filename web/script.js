window.addEventListener('DOMContentLoaded', () => {
    const canvas = document.getElementById('paintCanvas');
    if (!canvas) {
        console.error("Canvas not found");
        return;
    }

    const ctx = canvas.getContext('2d');
    const socket = new WebSocket(`ws://${window.location.host}/ws`);

    let painting = false;
    let lastPos = null;
    window.activeColor = '#39ff14';
    let isEraser = false;

    // 1. Get elements with safety checks
    const chatInput = document.getElementById('chatInput');
    const chatBox = document.getElementById('chatBox');
    const sendBtn = document.getElementById('sendBtn');

    // 2. The Sender Function
    function handleSendMessage() {
        if (!chatInput || !chatBox) {
            console.error("Chat elements missing from HTML!");
            return;
        }

        const text = chatInput.value.trim();
        if (text === "") return;

        if (socket.readyState !== WebSocket.OPEN) {
            console.warn("Socket not open. Current state:", socket.readyState);
            return;
        }

        const chatData = {
            type: 'chat',
            content: text,
            sender: "Guest"
        };

        console.log("Sending chat to server...", chatData);
        socket.send(JSON.stringify(chatData));
        chatInput.value = "";
    }

    // 3. The Receiver Logic
    socket.onmessage = (e) => {
        const msg = JSON.parse(e.data);

        if (msg.type === 'draw') {
            draw(msg.prevX, msg.prevY, msg.x, msg.y, msg.color, msg.size);
        }
        else if (msg.type === 'clear') {
            ctx.clearRect(0, 0, canvas.width, canvas.height);
        }
        else if (msg.type === 'chat') {
            console.info("Processing chat message UI update...");

            // Use direct selection to avoid 'null' variable issues
            const targetBox = document.getElementById('chatBox');

            if (!targetBox) {
                console.error("CRITICAL: Element #chatBox not found in the DOM!");
                return;
            }

            // Create the new message line
            const msgLine = document.createElement('div');
            msgLine.className = "chat-entry";
            msgLine.style.color = "#ffffff";
            msgLine.style.marginBottom = "5px";

            // Ensure we use the correct keys from the Go struct
            const name = msg.sender || "Guest";
            const text = msg.content || "";

            msgLine.innerHTML = `<strong style="color:#39ff14">${name}:</strong> <span>${text}</span>`;

            // Add to UI
            targetBox.appendChild(msgLine);

            // Scroll to bottom
            targetBox.scrollTop = targetBox.scrollHeight;

            console.info("Message successfully added to UI.");
        }
    };

    // 4. Hook up Events
    if (sendBtn) sendBtn.onclick = handleSendMessage;
    if (chatInput) {
        chatInput.onkeydown = (e) => {
            if (e.key === 'Enter') handleSendMessage();
        };
    }

    function initCanvas() {
        const rect = canvas.parentNode.getBoundingClientRect();
        canvas.width = rect.width;
        canvas.height = rect.height;
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
    }

    function getMousePos(e) {
        const rect = canvas.getBoundingClientRect();
        return { x: e.clientX - rect.left, y: e.clientY - rect.top };
    }

    canvas.onmousedown = (e) => { painting = true; lastPos = getMousePos(e); };
    window.onmouseup = () => { painting = false; lastPos = null; };

    canvas.onmousemove = (e) => {
        if (!painting) return;

        const pos = getMousePos(e);

        if (lastPos) {
            const color = isEraser ? '#000000' : window.activeColor;
            const size = isEraser ? 25 : 5;

            draw(lastPos.x, lastPos.y, pos.x, pos.y, color, size);

            if (socket.readyState === WebSocket.OPEN) {
                socket.send(JSON.stringify({
                    type: 'draw',
                    x: pos.x,
                    y: pos.y,
                    prevX: lastPos.x,
                    prevY: lastPos.y,
                    color,
                    size
                }));
            }
        }

        lastPos = pos;
    };

    function draw(x1, y1, x2, y2, color, size) {
        ctx.beginPath();
        ctx.strokeStyle = color;
        ctx.lineWidth = size;
        ctx.moveTo(x1, y1);
        ctx.lineTo(x2, y2);
        ctx.stroke();
    }

    window.setTool = (tool) => {
        isEraser = (tool === 'eraser');

        document.getElementById('pencilBtn').classList.toggle('active', !isEraser);
        document.getElementById('eraserBtn').classList.toggle('active', isEraser);
    };

    window.clearCanvas = () => {
        socket.send(JSON.stringify({ type: 'clear' }));
        ctx.clearRect(0, 0, canvas.width, canvas.height);
    };

    window.addEventListener('resize', initCanvas);

    initCanvas();
});