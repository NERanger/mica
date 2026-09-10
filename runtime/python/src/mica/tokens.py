from mica.method import RpcMethod

try:
    from mica._tokens_gen import *  # noqa: F403
except ImportError:
    pass

__all__ = ["RpcMethod"]
