FROM mongo

RUN echo "mysecret" > /mongo.key && \
    chmod 600 /mongo.key && \
    chown mongodb:mongodb /mongo.key