// Doodle Royale client. Connects to the Go WebSocket hub, renders incoming
// drawing/chat/word/timer events and publishes local drawing and chat actions.
window.addEventListener('DOMContentLoaded', () => {
    const canvas = document.getElementById('paintCanvas');
    if (!canvas) {
        console.error('Canvas element #paintCanvas not found');
        return;
    }
    const ctx = canvas.getContext('2d');

    // --- Configuration -------------------------------------------------------
    const PEN_SIZE = 5;
    const ERASER_SIZE = 25;
    const ERASER_COLOR = '#000000'; // matches the canvas background
    const DEFAULT_COLOR = '#39ff14';

    // --- Local state ---------------------------------------------------------
    const state = {
        painting: false,
        lastPos: null,
        color: DEFAULT_COLOR,
        eraser: false,
    };

    // --- WebSocket -----------------------------------------------------------
    const wsProtocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const socket = new WebSocket(`${wsProtocol}://${window.location.host}/ws`);

    function send(payload) {
        if (socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify(payload));
        }
    }

    socket.addEventListener('message', (event) => {
        let msg;
        try {
            msg = JSON.parse(event.data);
        } catch {
            return; // ignore malformed frames
        }
        handleServerMessage(msg);
    });

    function handleServerMessage(msg) {
        switch (msg.type) {
            case 'draw':
                drawSegment(msg);
                break;
            case 'clear':
                clearCanvas();
                break;
            case 'history':
                (msg.history || []).forEach(drawSegment);
                break;
            case 'chat':
                appendChat(msg.sender || 'Guest', msg.content || '');
                break;
            case 'timer':
                updateTimer(msg.content);
                break;
            case 'word':
                updateWord(msg.content || '');
                break;
        }
    }

    // --- Drawing -------------------------------------------------------------
    function drawSegment(seg) {
        ctx.beginPath();
        ctx.strokeStyle = seg.color;
        ctx.lineWidth = seg.size;
        ctx.moveTo(seg.prevX, seg.prevY);
        ctx.lineTo(seg.x, seg.y);
        ctx.stroke();
    }

    function clearCanvas() {
        ctx.clearRect(0, 0, canvas.width, canvas.height);
    }

    function getMousePos(e) {
        const rect = canvas.getBoundingClientRect();
        return { x: e.clientX - rect.left, y: e.clientY - rect.top };
    }

    canvas.addEventListener('mousedown', (e) => {
        state.painting = true;
        state.lastPos = getMousePos(e);
    });

    window.addEventListener('mouseup', () => {
        state.painting = false;
        state.lastPos = null;
    });

    canvas.addEventListener('mousemove', (e) => {
        if (!state.painting || !state.lastPos) return;

        const pos = getMousePos(e);
        const segment = {
            type: 'draw',
            prevX: state.lastPos.x,
            prevY: state.lastPos.y,
            x: pos.x,
            y: pos.y,
            color: state.eraser ? ERASER_COLOR : state.color,
            size: state.eraser ? ERASER_SIZE : PEN_SIZE,
        };

        drawSegment(segment); // draw locally for instant feedback
        send(segment);        // and share with everyone else
        state.lastPos = pos;
    });

    // --- Chat ----------------------------------------------------------------
    const chatInput = document.getElementById('chatInput');
    const chatBox = document.getElementById('chatBox');
    const sendBtn = document.getElementById('sendBtn');

    // appendChat builds the DOM with textContent so chat text can never be
    // interpreted as HTML (prevents cross-site scripting via chat messages).
    function appendChat(sender, text) {
        if (!chatBox) return;

        const line = document.createElement('div');
        line.className = 'chat-entry';

        const name = document.createElement('strong');
        name.style.color = DEFAULT_COLOR;
        name.textContent = `${sender}: `;

        const body = document.createElement('span');
        body.textContent = text;

        line.append(name, body);
        chatBox.appendChild(line);
        chatBox.scrollTop = chatBox.scrollHeight;
    }

    function sendChat() {
        if (!chatInput) return;
        const text = chatInput.value.trim();
        if (!text) return;
        send({ type: 'chat', content: text, sender: 'Guest' });
        chatInput.value = '';
    }

    if (sendBtn) sendBtn.addEventListener('click', sendChat);
    if (chatInput) {
        chatInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') sendChat();
        });
    }

    // --- Header: word slots and countdown -----------------------------------
    const wordSlots = document.querySelector('.word-slots');
    const clock = document.querySelector('.neon-text');

    function updateWord(word) {
        if (wordSlots) {
            wordSlots.textContent = word.replace(/[a-zA-Z]/g, '_ ').trim();
        }
    }

    function updateTimer(seconds) {
        if (!clock) return;
        const icon = document.createElement('i');
        icon.className = 'fas fa-clock';
        clock.replaceChildren(icon, ` ${seconds}s`);
    }

    // --- Toolbar -------------------------------------------------------------
    const pencilBtn = document.getElementById('pencilBtn');
    const eraserBtn = document.getElementById('eraserBtn');
    const colorPicker = document.getElementById('colorPicker');
    const clearBtn = document.getElementById('clearBtn');

    function setTool(useEraser) {
        state.eraser = useEraser;
        if (pencilBtn) pencilBtn.classList.toggle('active', !useEraser);
        if (eraserBtn) eraserBtn.classList.toggle('active', useEraser);
    }

    if (pencilBtn) pencilBtn.addEventListener('click', () => setTool(false));
    if (eraserBtn) eraserBtn.addEventListener('click', () => setTool(true));
    if (colorPicker) {
        colorPicker.addEventListener('input', (e) => { state.color = e.target.value; });
    }
    if (clearBtn) {
        clearBtn.addEventListener('click', () => {
            send({ type: 'clear' });
            clearCanvas();
        });
    }

    // --- Canvas sizing -------------------------------------------------------
    function resizeCanvas() {
        const rect = canvas.parentNode.getBoundingClientRect();
        canvas.width = rect.width;
        canvas.height = rect.height;
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
    }

    window.addEventListener('resize', resizeCanvas);
    resizeCanvas();
});
